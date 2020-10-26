package dulumi

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"strings"
)

func NewEcsCluster(ctx *plm.Context, service string) (*ecs.Cluster, error) {
	cluster, err := ecs.NewCluster(ctx, service, &ecs.ClusterArgs{
		Name: plm.StringPtr(service),
		Settings: ecs.ClusterSettingArray{
			ecs.ClusterSettingArgs{
				Name:  plm.String("containerInsights"),
				Value: plm.String("enabled"),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func ContainerEnvs(appEnvs map[string]string) []string {
	var envs []string
	for k, v := range appEnvs {
		envs = append(envs, ContainerEnv(k, v))
	}
	return envs
}

func ContainerEnv(key string, value string) string {
	return fmt.Sprintf(`{
	"name": "%v",
	"value": "%v"
	}`, key, value)
}

func ContainerSecret(secretArn *string, secret string) string {
	return fmt.Sprintf(`{
	"valueFrom": "%v:%v::",
	"name": "%v"
	}`, *secretArn, secret, secret)
}

func ContainerSecrets(appSecrets map[string]string, secretArn *string) []string {
	var secrets []string
	for k := range appSecrets {
		secrets = append(secrets, ContainerSecret(secretArn, k))
	}
	return secrets
}

func ContainerEnvJsonArray(envs []string) string {
	return strings.Join(envs, ",")
}
