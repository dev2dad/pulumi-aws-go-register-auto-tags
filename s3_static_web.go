package dulumi

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/cloudfront"
	build "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codebuild"
	pipeline "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codepipeline"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/route53"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/sallgoood/dulumi/utils"
)

type S3StaticWeb struct {
	plm.ResourceState

	BucketName     plm.StringOutput `pulumi:"bucketName"`
	DistributionId plm.IDOutput     `pulumi:"id"`
}

type S3StaticWebArgs struct {
	Product string `json:"product"`
	Env     string `json:"env"`

	CertificateArn string `json:"domain-ssl-cert_arn"`
	Domain         string `json:"domain"`
	SubDomain      string `json:"subdomain"`

	GitRepo   string `json:"git-repo"`
	GitBranch string `json:"git-branch"`

	CICDBuildRole           string              `json:"cicd-build-role"`
	CICDBuildEnvs           []map[string]string `json:"cicd-build-envs"`
	CICDPipelineRole        string              `json:"cicd-pipeline-role"`
	CICDGitPolling          bool                `json:"cicd-git-polling"`
	CICDRequireApproval     bool                `json:"cicd-require-approval"`
	CICDRequireNotification bool                `json:"cicd-require-notification"`
}

func NewS3StaticWeb(ctx *plm.Context, c *S3StaticWebArgs,
	opts ...plm.ResourceOption) (*S3StaticWeb, error) {
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
			"oAuthToken",
		})

	host := fmt.Sprintf("%v.%v", c.SubDomain, c.Domain)
	envProduct := fmt.Sprintf("%v-%v", c.Env, c.Product)
	productEnv := fmt.Sprintf("%v-%v", c.Product, c.Env)

	var dsw S3StaticWeb
	err := ctx.RegisterComponentResource("drama:web:s3-static", "drama-s3-static-web", &dsw, opts...)
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
		Acl:    plm.String("private"),
		Bucket: plm.String(host),
		Versioning: s3.BucketVersioningArgs{
			Enabled: plm.Bool(true),
		},
		Website: s3.BucketWebsiteArgs{
			RedirectAllRequestsTo: plm.String(fmt.Sprintf("https://%v", plm.String(host))),
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	zone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
		Name: &c.Domain,
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	originAccessIdentity, err := cloudfront.NewOriginAccessIdentity(ctx, "originAccessIdentity", &cloudfront.OriginAccessIdentityArgs{
		Comment: plm.String(host),
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	distribution, err := cloudfront.NewDistribution(ctx, "distribution", &cloudfront.DistributionArgs{
		Aliases: plm.StringArray{
			plm.String(host),
		},
		Origins: cloudfront.DistributionOriginArray{
			&cloudfront.DistributionOriginArgs{
				DomainName: bucket.BucketRegionalDomainName,
				OriginId:   plm.String(fmt.Sprintf("S3-%v", envProduct)),
				S3OriginConfig: &cloudfront.DistributionOriginS3OriginConfigArgs{
					OriginAccessIdentity: originAccessIdentity.CloudfrontAccessIdentityPath,
				},
			},
		},
		Restrictions: cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: plm.String("none"),
			},
		},
		Enabled:           plm.Bool(true),
		IsIpv6Enabled:     plm.Bool(true),
		Comment:           plm.String(host),
		DefaultRootObject: plm.String("index.html"),
		CustomErrorResponses: cloudfront.DistributionCustomErrorResponseArray{
			cloudfront.DistributionCustomErrorResponseArgs{
				ErrorCachingMinTtl: plm.IntPtr(0),
				ErrorCode:          plm.Int(403),
				ResponseCode:       plm.IntPtr(200),
				ResponsePagePath:   plm.StringPtr("/index.html"),
			},
		},
		DefaultCacheBehavior: cloudfront.DistributionDefaultCacheBehaviorArgs{
			AllowedMethods:       plm.StringArray{plm.String("GET"), plm.String("HEAD")},
			CachedMethods:        plm.StringArray{plm.String("GET"), plm.String("HEAD")},
			DefaultTtl:           plm.Int(86400),
			MinTtl:               plm.Int(0),
			MaxTtl:               plm.Int(31536000),
			TargetOriginId:       plm.String(fmt.Sprintf("S3-%v", envProduct)),
			ViewerProtocolPolicy: plm.String("redirect-to-https"),
			ForwardedValues: cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
				QueryString: plm.Bool(false),
				Cookies: cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{
					Forward: plm.String("none"),
				},
			},
		},
		ViewerCertificate: cloudfront.DistributionViewerCertificateArgs{
			AcmCertificateArn:      plm.String(c.CertificateArn),
			SslSupportMethod:       plm.String("sni-only"),
			MinimumProtocolVersion: plm.String("TLSv1"),
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	_, err = route53.NewRecord(ctx, "record", &route53.RecordArgs{
		Name:   plm.String(host),
		Type:   plm.String("A"),
		ZoneId: plm.String(zone.ZoneId),
		Aliases: route53.RecordAliasArray{
			&route53.RecordAliasArgs{
				Name:                 distribution.DomainName,
				ZoneId:               distribution.HostedZoneId,
				EvaluateTargetHealth: plm.Bool(true),
			},
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketPolicy(ctx, "bucketPolicy", &s3.BucketPolicyArgs{
		Bucket: bucket.Bucket,
		Policy: plm.Any(map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Principal": map[string]interface{}{
						"AWS": plm.Sprintf("%s", originAccessIdentity.IamArn),
					},
					"Action": []interface{}{
						"s3:GetObject",
					},
					"Resource": []interface{}{
						plm.Sprintf("%s/*", bucket.Arn),
					},
				},
			},
		}),
	}, plm.Parent(&dsw)); err != nil {
		return nil, err
	}

	_, err = s3.NewBucket(ctx, "cicd-bucket", &s3.BucketArgs{
		Bucket: plm.String(fmt.Sprintf("%v-cicd", envProduct)),
		Acl:    plm.String("private"),
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	buildEnvs := build.ProjectEnvironmentEnvironmentVariableArray{
		build.ProjectEnvironmentEnvironmentVariableArgs{
			Name:  plm.String("S3_BUCKET"),
			Type:  plm.String("PLAINTEXT"),
			Value: bucket.Bucket,
		},
		build.ProjectEnvironmentEnvironmentVariableArgs{
			Name:  plm.String("DISTRIBUTION_ID"),
			Type:  plm.String("PLAINTEXT"),
			Value: distribution.ID(),
		},
	}

	buildEnvs = AppendBuildEnvs(c.CICDBuildEnvs, buildEnvs)

	_, err = build.NewProject(ctx, "codebuild", &build.ProjectArgs{
		Artifacts: build.ProjectArtifactsArgs{
			Type: plm.String("CODEPIPELINE"),
		},
		Environment: build.ProjectEnvironmentArgs{
			ComputeType:          plm.String("BUILD_GENERAL1_SMALL"),
			Image:                plm.String("aws/codebuild/amazonlinux2-x86_64-standard:3.0"),
			PrivilegedMode:       plm.Bool(true),
			Type:                 plm.String("LINUX_CONTAINER"),
			EnvironmentVariables: buildEnvs,
		},
		Name:        plm.String(productEnv),
		ServiceRole: plm.String(c.CICDBuildRole),
		Source: build.ProjectSourceArgs{
			Buildspec: plm.String(
				`
version: 0.2
phases:
  install:
    runtime-versions:
      nodejs: 12
    commands:
      - echo Install yarn
      - npm install --global yarn
      - echo install dependencies
      - yarn
  build:
    commands:
      - echo Building the Vue app
      - yarn build
  post_build:
    commands:
      - echo Check BUILD_SUCCEEDING
      - if [ ${CODEBUILD_BUILD_SUCCEEDING} -eq 0 ]; then echo "BUILD FAILS" 1>&2; exit 1; fi
      - echo upload S3
      - aws s3 sync ./dist s3://$S3_BUCKET --delete
      - aws s3 cp ./dist/index.html s3://$S3_BUCKET/index.html --cache-control no-cache
      - aws cloudfront create-invalidation --distribution-id $DISTRIBUTION_ID --paths "/*"
`),
			Type: plm.String("CODEPIPELINE"),
		},
	}, plm.Parent(&dsw))
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
		},
	}, plm.Parent(&dsw),
		plm.IgnoreChanges([]string{"oAuthToken"})); err != nil {
		return nil, err
	}

	dsw.BucketName = bucket.Bucket
	dsw.DistributionId = distribution.ID()
	if err = ctx.RegisterResourceOutputs(&dsw, plm.Map{
		"bucketName":     bucket.Bucket,
		"distributionId": distribution.ID(),
	}); err != nil {
		return nil, err
	}

	return &dsw, nil
}
