package dulumi

import (
	"fmt"
	build "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codebuild"
	pipeline "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codepipeline"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"

	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type S3StaticWebCICD struct {
	plm.ResourceState

	bucketName plm.IDOutput `pulumi:"bucketName"`
}

func NewS3StaticWebCICD(ctx *plm.Context,
	webSiteBucket string,
	serviceEnv string,
	buildRole string,
	pipelineRole string,
	gitRepo string,
	gitBranch string,
	gitPolling bool,
	requireApproval bool,
	requireNoti bool,
	buildSpec string,
	opts ...plm.ResourceOption) (*S3StaticWebCICD, error) {

	var cicd S3StaticWebCICD
	err := ctx.RegisterComponentResource("drama:web:s3-static-cicd", "drama-s3-static-web-cicd", &cicd, opts...)
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
		Bucket: plm.String(fmt.Sprintf("%v-cicd", serviceEnv)),
		Acl:    plm.String("private"),
	}, plm.Parent(&cicd))
	if err != nil {
		return nil, err
	}

	buildPrj, err := build.NewProject(ctx, "codebuild", &build.ProjectArgs{
		Artifacts: build.ProjectArtifactsArgs{
			Type: plm.String("CODEPIPELINE"),
		},
		Environment: build.ProjectEnvironmentArgs{
			ComputeType:    plm.String("BUILD_GENERAL1_SMALL"),
			Image:          plm.String("aws/codebuild/amazonlinux2-x86_64-standard:3.0"),
			PrivilegedMode: plm.Bool(true),
			Type:           plm.String("LINUX_CONTAINER"),
		},
		Name:        plm.String(serviceEnv),
		ServiceRole: plm.String(buildRole),
		Source: build.ProjectSourceArgs{
			Buildspec: plm.String(buildSpec),
			Type:      plm.String("CODEPIPELINE"),
		},
	}, plm.Parent(&cicd))
	if err != nil {
		return nil, err
	}

	if _, err := pipeline.NewPipeline(ctx, "codepipeline", &pipeline.PipelineArgs{
		Name:    plm.String(serviceEnv),
		RoleArn: plm.String(pipelineRole),
		ArtifactStore: pipeline.PipelineArtifactStoreArgs{
			Location: bucket.Bucket,
			Type:     plm.String("S3"),
		},
		Stages: pipeline.PipelineStageArray{
			NewGithubSourceStage(gitRepo, gitBranch, gitPolling),
			NewCodebuildStage(fmt.Sprintf("%v", buildPrj.Name), requireApproval, requireNoti, gitRepo),
		},
	}, plm.Parent(&cicd),
		plm.IgnoreChanges([]string{"oAuthToken"})); err != nil {
		return nil, err
	}

	return &cicd, nil
}
