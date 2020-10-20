package dulumi

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	elb "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/elasticloadbalancingv2"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/sallgoood/dulumi/utils"
)

type FargateAPI struct {
	plm.ResourceState
}

func NewDulumiFargateAPI(ctx *plm.Context,
	service string,
	subnetIds []string,
	securityGroupIds []string,
	vpcId string,
	taskRole string,
	opts ...plm.ResourceOption,
) (*FargateAPI, error) {

	var dfa FargateAPI
	err := ctx.RegisterComponentResource("drama:server:fargate-api", "drama-fargate-api", &dfa, opts...)
	if err != nil {
		return nil, err
	}

	cluster, err := ecs.NewCluster(ctx, service, &ecs.ClusterArgs{
		Name: plm.StringPtr(service),
		Settings: ecs.ClusterSettingArray{
			ecs.ClusterSettingArgs{
				Name:  plm.String("containerInsights"),
				Value: plm.String("enabled"),
			},
		},
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	lb, err := elb.NewLoadBalancer(ctx, "lb", &elb.LoadBalancerArgs{
		Subnets:        utils.ToPulumiStringArray(subnetIds),
		SecurityGroups: utils.ToPulumiStringArray(securityGroupIds),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	tg, err := elb.NewTargetGroup(ctx, "tg", &elb.TargetGroupArgs{
		Port:       plm.Int(80),
		Protocol:   plm.String("HTTP"),
		TargetType: plm.String("ip"),
		VpcId:      plm.String(vpcId),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	listener, err := elb.NewListener(ctx, "listener", &elb.ListenerArgs{
		LoadBalancerArn: lb.Arn,
		Port:            plm.Int(80),
		DefaultActions: elb.ListenerDefaultActionArray{
			elb.ListenerDefaultActionArgs{
				Type:           plm.String("forward"),
				TargetGroupArn: tg.Arn,
			},
		},
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}

	containerDef := `[{
				"name": "app",
				"image": %q,
				"portMappings": [{
					"containerPort": 80,
					"hostPort": 80,
					"protocol": "tcp"
				}]
			}]`

	task, err := ecs.NewTaskDefinition(ctx, "app-task", &ecs.TaskDefinitionArgs{
		Family:                  plm.String("fargate-task-definition"),
		Cpu:                     plm.String("256"),
		Memory:                  plm.String("512"),
		NetworkMode:             plm.String("awsvpc"),
		RequiresCompatibilities: plm.StringArray{plm.String("FARGATE")},
		ExecutionRoleArn:        plm.String(taskRole),
		ContainerDefinitions:    plm.String(fmt.Sprintf(containerDef, "nginx")),
	}, plm.Parent(&dfa))
	if err != nil {
		return nil, err
	}
	_, err = ecs.NewService(ctx, "app-svc", &ecs.ServiceArgs{
		Cluster:        cluster.Arn,
		DesiredCount:   plm.Int(5),
		LaunchType:     plm.String("FARGATE"),
		TaskDefinition: task.Arn,
		NetworkConfiguration: &ecs.ServiceNetworkConfigurationArgs{
			AssignPublicIp: plm.Bool(true),
			Subnets:        utils.ToPulumiStringArray(subnetIds),
			SecurityGroups: utils.ToPulumiStringArray(securityGroupIds),
		},
		LoadBalancers: ecs.ServiceLoadBalancerArray{
			ecs.ServiceLoadBalancerArgs{
				TargetGroupArn: tg.Arn,
				ContainerName:  plm.String("app"),
				ContainerPort:  plm.Int(80),
			},
		},
	}, plm.DependsOn([]plm.Resource{listener}),
		plm.IgnoreChanges([]string{"TaskDefinition"}),
		plm.Parent(&dfa))

	return &dfa, nil
}