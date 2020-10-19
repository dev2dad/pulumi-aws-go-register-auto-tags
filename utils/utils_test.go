package utils

import (
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

type mocks int

func (mocks) NewResource(typeToken, name string, inputs resource.PropertyMap, provider, id string) (string, resource.PropertyMap, error) {
	return name + "_id", inputs, nil
}

func (mocks) Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	return args, nil
}

func TestRegisterAutoTags(t *testing.T) {
	err := plm.RunErr(func(ctx *plm.Context) error {
		RegisterAutoTags(ctx, plm.StringMap{
			"Environment": plm.String("test"),
		})
		taggable, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
			Tags: plm.StringMap{
				"Env": plm.String("test"),
			},
		})
		assert.NoError(t, err)

		taggableNoTag, err := s3.NewBucket(ctx, "bucket-no-tag", &s3.BucketArgs{})
		assert.NoError(t, err)

		_, err = cloudfront.NewOriginAccessIdentity(ctx, "originAccessIdentity", &cloudfront.OriginAccessIdentityArgs{})
		assert.NoError(t, err)

		var wg sync.WaitGroup
		wg.Add(1)

		// Test if the service has tags and a name tag.
		plm.All(taggable.Tags, taggableNoTag.Tags).ApplyT(func(all []interface{}) error {
			tags := all[0].(map[string]string)
			tagsNotTagged := all[1].(map[string]string)

			assert.Containsf(t, tags, "Environment", "missing a Environment tag")
			assert.Containsf(t, tags, "Env", "missing a Env tag")
			assert.Containsf(t, tagsNotTagged, "Environment", "missing a Environment tag")
			assert.NotContainsf(t, tagsNotTagged, "Env", "should be missing a Env tag")
			wg.Done()
			return nil
		})

		wg.Wait()
		return nil
	}, plm.WithMocks("project", "stack", mocks(0)))
	assert.NoError(t, err)
}
