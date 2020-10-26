package dulumi

import (
	build "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/codebuild"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func AppendBuildEnvs(envs []map[string]string, buildEnvs build.ProjectEnvironmentEnvironmentVariableArray) build.ProjectEnvironmentEnvironmentVariableArray {
	for _, v := range envs {
		buildEnvs = append(buildEnvs, build.ProjectEnvironmentEnvironmentVariableArgs{
			Name:  plm.String(v["name"]),
			Type:  plm.String(v["type"]),
			Value: plm.String(v["value"]),
		})
	}
	return buildEnvs
}
