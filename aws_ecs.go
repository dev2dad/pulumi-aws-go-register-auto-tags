package dulumi

import (
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
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
