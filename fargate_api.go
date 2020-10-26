package dulumi

import (
	"encoding/json"
	"fmt"
	aas "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/appautoscaling"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/cloudwatch"
	build "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codebuild"
	pipeline "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codepipeline"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecr"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	alb "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/lb"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/route53"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"
	scm "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/secretsmanager"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/sallgoood/dulumi/utils"
	"log"
	"strconv"
)

type FargateApi struct {
	plm.ResourceState

	Dns plm.StringOutput `pulumi:"dnsName"`
}

type FargateApiArgs struct {
	Product string `json:"product"`
	Env     string `json:"env"`
	EnvLong string `json:"env-long"`
	VPCId   string `json:"vpc-id"`

	LBSubnetIPs        []string `json:"lb-subnet-ids"`
	LBSecurityGroupIds []string `json:"lb-security-group-ids"`

	LBCertificateArn string `json:"lb-certificate-arn"`
	LBDomain         string `json:"lb-domain"`
	LBSubDomain      string `json:"lb-subdomain"`

	ECSTaskSubnetIds        []string `json:"ecs-task-subnet-ids"`
	ECSTaskSecurityGroupIds []string `json:"ecs-task-security-group-ids"`
	ECSTaskRole             string   `json:"ecs-task-role"`
	ECSExecutionRole        string   `json:"ecs-execution-role"`

	AppPort            int               `json:"app-port"`
	AppSecrets         map[string]string `json:"app-secrets"`
	AppEnvs            map[string]string `json:"app-envs"`
	AppHealthCheckPath string            `json:"app-health-check-path"`
	AppScaleCpuPercent float64           `json:"app-scale-cpu-percent"`
	AppScaleMin        int               `json:"app-scale-min"`
	AppScaleMax        int               `json:"app-scale-max"`
	AppCpu             string            `json:"app-cpu"`
	AppMemory          string            `json:"app-memory"`
	AppEnableLogRouter bool              `json:"app-enable-logrouter"`

	GitRepo   string `json:"git-repo"`
	GitBranch string `json:"git-branch"`

	CICDBuildRole           string              `json:"cicd-build-role"`
	CICDPipelineRole        string              `json:"cicd-pipeline-role"`
	CICDGitPolling          bool                `json:"cicd-git-polling"`
	CICDRequireApproval     bool                `json:"cicd-require-approval"`
	CICDRequireNotification bool                `json:"cicd-require-notification"`
	CICDBuildEnvs           []map[string]string `json:"cicd-build-envs"`
}

func NewFargateApi(ctx *plm.Context, c FargateApiArgs,
	opts ...plm.ResourceOption,
) (*FargateApi, error) {
	utils.RegisterAutoTags(ctx, plm.StringMap{
		"Role":        plm.String("infra"),
		"Environment": plm.String(c.Env),
		"Service":     plm.String(c.Product),
		"Team":        plm.String("dev"),
	})

	utils.IgnoreChanges(ctx,
		true,
		nil,
		[]string{
			"taskDefinition",
			"oAuthToken",
			"containerDefinitions",
			"desiredCount",
		})

	var dfa FargateApi
	err := ctx.RegisterComponentResource("drama:server:fargate-api", "drama-fargate-api", &dfa, opts...)
	if err != nil {
		return nil, err
	}

	productEnv := fmt.Sprintf("%v-%v", c.Product, c.Env)

	cluster, err := ecs.LookupCluster(ctx, &ecs.LookupClusterArgs{
		ClusterName: c.Product,
	})
	if err != nil {
		return nil, err
	}

	lb, err := alb.NewLoadBalancer(ctx, "alb", &alb.LoadBalancerArgs{
		Name:           plm.String(productEnv),
		Subnets:        utils.ToPulumiStringArray(c.LBSubnetIPs),
		SecurityGroups: utils.ToPulumiStringArray(c.LBSecurityGroupIds),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	tg, err := alb.NewTargetGroup(ctx, "targetGroup", &alb.TargetGroupArgs{
		Name:                plm.String(productEnv),
		Port:                plm.Int(c.AppPort),
		Protocol:            plm.String("HTTP"),
		TargetType:          plm.String("ip"),
		VpcId:               plm.String(c.VPCId),
		DeregistrationDelay: plm.Int(1),
		HealthCheck: alb.TargetGroupHealthCheckArgs{
			Enabled:            plm.BoolPtr(true),
			HealthyThreshold:   plm.IntPtr(3),
			UnhealthyThreshold: plm.IntPtr(3),
			Interval:           plm.IntPtr(30),
			Matcher:            plm.StringPtr("200-399"),
			Path:               plm.StringPtr(c.AppHealthCheckPath),
			Port:               plm.StringPtr(strconv.Itoa(c.AppPort)),
			Protocol:           plm.StringPtr("HTTP"),
			Timeout:            plm.IntPtr(5),
		},
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	_, err = alb.NewListener(ctx, "httpListener", &alb.ListenerArgs{
		LoadBalancerArn: lb.Arn,
		Port:            plm.Int(80),
		DefaultActions: alb.ListenerDefaultActionArray{
			alb.ListenerDefaultActionArgs{
				Type: plm.String("redirect"),
				Redirect: alb.ListenerDefaultActionRedirectArgs{
					Port:       plm.StringPtr("443"),
					Protocol:   plm.StringPtr("HTTPS"),
					StatusCode: plm.String("HTTP_301"),
				},
			},
		},
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	https, err := alb.NewListener(
		ctx,
		"httpsListener",
		NewSimpleForwardingHttpsListener(lb, tg, c.LBCertificateArn),
		plm.Parent(&dfa),
	)
	if err != nil {
		return nil, err
	}

	logGroup, err := cloudwatch.NewLogGroup(ctx, "logGroup", &cloudwatch.LogGroupArgs{
		Name:            plm.String(productEnv),
		RetentionInDays: plm.IntPtr(30),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	secretManager, err := scm.NewSecret(ctx, "secretManager", &scm.SecretArgs{
		Name: plm.String(productEnv),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	secretJson, err := json.Marshal(c.AppSecrets)
	if err != nil {
		return nil, err
	}

	_, err = scm.NewSecretVersion(ctx, fmt.Sprintf("secrets"), &scm.SecretVersionArgs{
		SecretId:     secretManager.ID(),
		SecretString: plm.ToSecret(plm.String(secretJson)).(plm.StringOutput),
	}, plm.Parent(&dfa),
		plm.DependsOn([]plm.Resource{secretManager}))
	if err != nil {
		return nil, err
	}

	var secretArn string
	secretM, err := scm.LookupSecret(ctx, &scm.LookupSecretArgs{
		Name: &productEnv,
	}, plm.Parent(&dfa))
	if err != nil {
		log.Println(fmt.Sprintf("secretmanager, %v is not ready yet", productEnv))
	} else {
		secretArn = secretM.Arn
	}

	initialTask, err := ecs.NewTaskDefinition(ctx, "ecsTaskDefinition", &ecs.TaskDefinitionArgs{
		Family:                  plm.String(productEnv),
		Cpu:                     plm.String(c.AppCpu),
		Memory:                  plm.String(c.AppMemory),
		NetworkMode:             plm.String("awsvpc"),
		RequiresCompatibilities: plm.StringArray{plm.String("FARGATE")},
		TaskRoleArn:             plm.String(c.ECSTaskRole),
		ExecutionRoleArn:        plm.String(c.ECSExecutionRole),
		ContainerDefinitions: plm.String(ContainerDefinitionTemplate(
			fmt.Sprintf(
				"%v/%v:latest",
				"784015586554.dkr.ecr.ap-northeast-1.amazonaws.com",
				productEnv,
			),
			strconv.Itoa(c.AppPort),
			productEnv,
			c.AppEnvs,
			&secretArn,
			c.AppSecrets,
			c.AppEnableLogRouter,
		)),
	}, plm.Parent(&dfa),
		plm.DependsOn([]plm.Resource{logGroup, secretManager}))
	if err != nil {
		return nil, err
	}

	svc, err := ecs.NewService(ctx, "ecsService", &ecs.ServiceArgs{
		Name:           plm.String(productEnv),
		Cluster:        plm.String(cluster.Arn),
		TaskDefinition: initialTask.Arn,
		DesiredCount:   plm.Int(1),
		LaunchType:     plm.String("FARGATE"),
		DeploymentController: ecs.ServiceDeploymentControllerArgs{
			Type: plm.StringPtr("ECS"),
		},
		NetworkConfiguration: &ecs.ServiceNetworkConfigurationArgs{
			AssignPublicIp: plm.Bool(true),
			Subnets:        utils.ToPulumiStringArray(c.ECSTaskSubnetIds),
			SecurityGroups: utils.ToPulumiStringArray(c.ECSTaskSecurityGroupIds),
		},
		LoadBalancers: ecs.ServiceLoadBalancerArray{
			ecs.ServiceLoadBalancerArgs{
				TargetGroupArn: tg.Arn,
				ContainerName:  plm.String("app"),
				ContainerPort:  plm.Int(c.AppPort),
			},
		},
	}, plm.DependsOn([]plm.Resource{https}),
		plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	autoscaleResourceId := plm.String(fmt.Sprintf("service/%v/%v", cluster.ClusterName, productEnv))

	_, err = aas.NewTarget(ctx, "autoscaleTarget", &aas.TargetArgs{
		MaxCapacity:       plm.Int(c.AppScaleMax),
		MinCapacity:       plm.Int(c.AppScaleMin),
		ResourceId:        autoscaleResourceId,
		ScalableDimension: plm.String("ecs:service:DesiredCount"),
		ServiceNamespace:  plm.String("ecs"),
	}, plm.Parent(&dfa),
		plm.DependsOn([]plm.Resource{svc}))
	if err != nil {
		return nil, err
	}

	_, err = aas.NewPolicy(ctx, "autoscalePolicy", &aas.PolicyArgs{
		Name:              plm.String("scale-inout"),
		PolicyType:        plm.String("TargetTrackingScaling"),
		ResourceId:        autoscaleResourceId,
		ScalableDimension: plm.String("ecs:service:DesiredCount"),
		ServiceNamespace:  plm.String("ecs"),
		TargetTrackingScalingPolicyConfiguration: aas.PolicyTargetTrackingScalingPolicyConfigurationArgs{
			CustomizedMetricSpecification: nil,
			DisableScaleIn:                nil,
			PredefinedMetricSpecification: aas.PolicyTargetTrackingScalingPolicyConfigurationPredefinedMetricSpecificationArgs{
				PredefinedMetricType: plm.String("ECSServiceAverageCPUUtilization"),
			},
			ScaleInCooldown:  plm.IntPtr(30),
			ScaleOutCooldown: plm.IntPtr(1),
			TargetValue:      plm.Float64(c.AppScaleCpuPercent),
		},
	}, plm.Parent(&dfa),
		plm.DependsOn([]plm.Resource{svc}))
	if err != nil {
		return nil, err
	}

	if c.LBCertificateArn != "" {
		zone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
			Name: &c.LBDomain,
		}, plm.Parent(&dfa))
		if err != nil {
			return nil, err
		}

		_, err = route53.NewRecord(ctx, "record", &route53.RecordArgs{
			Name:   plm.String(fmt.Sprintf("%v.%v", c.LBSubDomain, c.LBDomain)),
			Type:   plm.String("A"),
			ZoneId: plm.String(zone.ZoneId),
			Aliases: route53.RecordAliasArray{
				&route53.RecordAliasArgs{
					Name:                 lb.DnsName,
					ZoneId:               lb.ZoneId,
					EvaluateTargetHealth: plm.Bool(true),
				},
			},
		}, plm.Parent(&dfa))
		if err != nil {
			return nil, err
		}
	}

	ecrRepo, err := ecr.NewRepository(ctx, "ecr", &ecr.RepositoryArgs{
		Name: plm.String(productEnv),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	_, err = ecr.NewLifecyclePolicy(ctx, "ecrLifecycle", &ecr.LifecyclePolicyArgs{
		Policy:     plm.String(ECR_LIFECYCLE_POLICY),
		Repository: ecrRepo.Name,
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
		Bucket: plm.String(fmt.Sprintf("%v-cicd", productEnv)),
		Acl:    plm.String("private"),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	_, err = build.NewProject(ctx, "codebuild", &build.ProjectArgs{
		Artifacts: build.ProjectArtifactsArgs{
			Type: plm.String("CODEPIPELINE"),
		},
		Environment: build.ProjectEnvironmentArgs{
			ComputeType:          plm.String("BUILD_GENERAL1_SMALL"),
			Image:                plm.String("aws/codebuild/amazonlinux2-x86_64-standard:3.0"),
			PrivilegedMode:       plm.Bool(true),
			Type:                 plm.String("LINUX_CONTAINER"),
			EnvironmentVariables: AppendBuildEnvs(c.CICDBuildEnvs, build.ProjectEnvironmentEnvironmentVariableArray{}),
		},
		Name:        plm.String(productEnv),
		ServiceRole: plm.String(c.CICDBuildRole),
		Source: build.ProjectSourceArgs{
			Buildspec: plm.String(BuildSpecTemplate(productEnv)),
			Type:      plm.String("CODEPIPELINE"),
		},
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	if _, err := pipeline.NewPipeline(ctx, "codepipeline", &pipeline.PipelineArgs{
		Name:    plm.String(productEnv),
		RoleArn: plm.String(c.CICDPipelineRole),
		ArtifactStore: pipeline.PipelineArtifactStoreArgs{
			Location: bucket.Bucket,
			Type:     plm.String("S3"),
		},
		Stages: pipeline.PipelineStageArray{
			NewGithubSourceStage(c.GitRepo, c.GitBranch, c.CICDGitPolling),
			NewCodebuildStage(productEnv, c.CICDRequireApproval),
			fargateApiCD(c.Product, productEnv),
		},
	}, plm.Parent(&dfa),
		plm.DependsOn([]plm.Resource{ecrRepo}),
		plm.IgnoreChanges([]string{"oAuthToken"})); err != nil {
		return nil, err
	}

	if err = ctx.RegisterResourceOutputs(&dfa, plm.Map{
		"dns": lb.DnsName,
	}); err != nil {
		return nil, err
	}

	dfa.Dns = lb.DnsName

	return &dfa, nil
}

func NewSimpleForwardingHttpsListener(
	lb *alb.LoadBalancer,
	tg *alb.TargetGroup,
	certificateArn string,
) *alb.ListenerArgs {

	https := alb.ListenerArgs{
		LoadBalancerArn: lb.Arn,
		Protocol:        plm.String("HTTPS"),
		Port:            plm.Int(443),
		DefaultActions: alb.ListenerDefaultActionArray{
			alb.ListenerDefaultActionArgs{
				Type:           plm.String("forward"),
				TargetGroupArn: tg.Arn,
			},
		},
	}

	if certificateArn != "" {
		https.CertificateArn = plm.StringPtr(certificateArn)
	}
	return &https
}

func fargateApiCD(
	ecsCluster string,
	ecsService string,
) pipeline.PipelineStageArgs {
	actions := pipeline.PipelineStageActionArray{}
	actions = AddECSDeployAction(actions, ecsCluster, ecsService)

	return pipeline.PipelineStageArgs{
		Name:    plm.String("Deploy"),
		Actions: actions,
	}
}
