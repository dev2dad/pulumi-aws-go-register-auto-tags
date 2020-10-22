package dulumi

import (
	"fmt"
	pipeline "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codepipeline"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func AddNotifyStageAction(actions pipeline.PipelineStageActionArray, gitRepo string) pipeline.PipelineStageActionArray {
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

func AddManualApprovalAction(actions pipeline.PipelineStageActionArray) pipeline.PipelineStageActionArray {
	return append(actions, pipeline.PipelineStageActionArgs{
		Name:     plm.String("Approval"),
		Category: plm.String("Approval"),
		Owner:    plm.String("AWS"),
		Provider: plm.String("Manual"),
		Version:  plm.String("1"),
	})
}

func AddS3DeployAction(actions pipeline.PipelineStageActionArray, webSiteBucket string) pipeline.PipelineStageActionArray {
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

func AddECSDeployAction(actions pipeline.PipelineStageActionArray, ecsCluster string, ecsService string) pipeline.PipelineStageActionArray {
	return append(actions, pipeline.PipelineStageActionArgs{
		Name:     plm.String("Deploy"),
		Category: plm.String("Deploy"),
		Configuration: plm.StringMap{
			"ClusterName": plm.String(ecsCluster),
			"ServiceName": plm.String(ecsService),
			"FileName":    plm.String("imagedefinitions.json"),
		},
		InputArtifacts: plm.StringArray{plm.String("BuildArtifact")},
		Owner:          plm.String("AWS"),
		Provider:       plm.String("ECS"),
		Version:        plm.String("1"),
	})
}

func AddGithubSourceAction(actions pipeline.PipelineStageActionArray, gitRepo string, gitBranch string, gitPolling bool) pipeline.PipelineStageActionArray {
	return append(actions, pipeline.PipelineStageActionArgs{
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
	})
}

func AddCodeBuildAction(actions pipeline.PipelineStageActionArray, buildProjectName string) pipeline.PipelineStageActionArray {
	return append(actions, pipeline.PipelineStageActionArgs{
		Name:     plm.String("Build"),
		Category: plm.String("Build"),
		Configuration: plm.StringMap{
			"ProjectName": plm.String(buildProjectName),
		},
		InputArtifacts:  plm.StringArray{plm.String("SourceArtifact")},
		OutputArtifacts: plm.StringArray{plm.String("BuildArtifact")},
		Owner:           plm.String("AWS"),
		Provider:        plm.String("CodeBuild"),
		Version:         plm.String("1"),
	})
}

func NewGithubSourceStage(gitRepo string, gitBranch string, gitPolling bool) pipeline.PipelineStageArgs {
	actions := pipeline.PipelineStageActionArray{}
	actions = AddGithubSourceAction(actions, gitRepo, gitBranch, gitPolling)

	return pipeline.PipelineStageArgs{
		Name:    plm.String("Source"),
		Actions: actions,
	}
}

func NewCodebuildStage(buildProjectName string, approval bool) pipeline.PipelineStageArgs {
	actions := pipeline.PipelineStageActionArray{}

	actions = AddCodeBuildAction(actions, buildProjectName)

	if approval {
		actions = AddManualApprovalAction(actions)
	}

	return pipeline.PipelineStageArgs{
		Name:    plm.String("Build"),
		Actions: actions,
	}
}
