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
			BuildPipe(gitRepo, gitBranch, gitPolling, buildPrj),
			DeployPipe(webSiteBucket, gitRepo, requireApproval, requireNoti),
		},
	}, plm.Parent(&cicd),
		plm.IgnoreChanges([]string{"oAuthToken"})); err != nil {
		return nil, err
	}

	return &cicd, nil
}

func BuildPipe(
	gitRepo string,
	gitBranch string,
	gitPolling bool,
	buildPrj *build.Project,
) pipeline.PipelineStageArgs {
	return pipeline.PipelineStageArgs{
		Name: plm.String("CI"),
		Actions: pipeline.PipelineStageActionArray{
			pipeline.PipelineStageActionArgs{
				Name:            plm.String("Source"),
				Category:        plm.String("Source"),
				Owner:           plm.String("ThirdParty"),
				Provider:        plm.String("GitHub"),
				Version:         plm.String("1"),
				OutputArtifacts: plm.StringArray{plm.String("SourceArtifact")},
				Configuration: plm.StringMap{
					"OAuthToken":           plm.String("invalidTemporaryToken"),
					"Owner":                plm.String("dramancompany"),
					"Repo":                 plm.String(gitRepo),
					"Branch":               plm.String(gitBranch),
					"PollForSourceChanges": plm.String(fmt.Sprintf("%v", gitPolling)),
				},
			},
			pipeline.PipelineStageActionArgs{
				Name:     plm.String("Build"),
				Category: plm.String("Build"),
				Configuration: plm.StringMap{
					"ProjectName": buildPrj.Name,
				},
				InputArtifacts:  plm.StringArray{plm.String("SourceArtifact")},
				OutputArtifacts: plm.StringArray{plm.String("BuildArtifact")},
				Owner:           plm.String("AWS"),
				Provider:        plm.String("CodeBuild"),
				Version:         plm.String("1"),
			},
		},
	}
}

func DeployPipe(
	webSiteBucket string,
	gitRepo string,
	approval bool,
	noti bool,
) pipeline.PipelineStageArgs {
	addApproval := func(actions pipeline.PipelineStageActionArray) pipeline.PipelineStageActionArray {
		return append(actions, pipeline.PipelineStageActionArgs{
			Name:     plm.String("Approval"),
			Category: plm.String("Approval"),
			Owner:    plm.String("AWS"),
			Provider: plm.String("Manual"),
			Version:  plm.String("1"),
		})
	}

	addDeploy := func(actions pipeline.PipelineStageActionArray) pipeline.PipelineStageActionArray {
		return append(actions, pipeline.PipelineStageActionArgs{
			Name:     plm.String("Deploy"),
			Category: plm.String("Deploy"),
			Configuration: plm.StringMap{
				"BucketName": plm.String(webSiteBucket),
				"Extract":    plm.String("true"),
			},
			InputArtifacts: plm.StringArray{plm.String("BuildArtifact")},
			Owner:          plm.String("AWS"),
			Provider:       plm.String("S3"),
			Version:        plm.String("1"),
		})
	}

	addNofi := func(actions pipeline.PipelineStageActionArray) pipeline.PipelineStageActionArray {
		return append(actions,
			pipeline.PipelineStageActionArgs{
				Name:           plm.String("Notify"),
				Category:       plm.String("Invoke"),
				InputArtifacts: plm.StringArray{plm.String("SourceArtifact")},
				Owner:          plm.String("AWS"),
				Provider:       plm.String("Lambda"),
				Configuration: plm.StringMap{
					"FunctionName": plm.String("code-pipeline-production"),
					"UserParameters": plm.String(fmt.Sprintf(`{
							"owner"       : "%v"
							"repo"        : "%v"
							"serviceName" : "%v"
						}`, "dramancompany", gitRepo, "Remember")),
				},
				Version: plm.String("1"),
			})
	}

	actions := pipeline.PipelineStageActionArray{}
	if approval {
		actions = addApproval(actions)
	}
	actions = addDeploy(actions)
	if noti {
		actions = addNofi(actions)
	}

	return pipeline.PipelineStageArgs{
		Name:    plm.String("CD"),
		Actions: actions,
	}

}
