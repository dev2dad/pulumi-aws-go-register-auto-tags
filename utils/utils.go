package utils

import (
	"log"
	"reflect"

	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func RegisterAutoTags(ctx *plm.Context, autoTags map[string]string) {
	err := ctx.RegisterStackTransformation(
		func(args *plm.ResourceTransformationArgs) *plm.ResourceTransformationResult {
			ptr := reflect.ValueOf(args.Props)
			if !ptr.IsZero() {
				val := ptr.Elem()
				tags := val.FieldByName("Tags")

				if tags.IsValid() {

					var tagsMap plm.Map
					if !tags.IsZero() {
						tagsMap = tags.Interface().(plm.Map)
					} else {
						tagsMap = map[string]plm.Input{}
					}
					for k, v := range autoTags {
						tagsMap[k] = plm.String(v)
					}
					tags.Set(reflect.ValueOf(tagsMap))

					return &plm.ResourceTransformationResult{
						Props: args.Props,
						Opts:  args.Opts,
					}
				} else {
					return nil
				}
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal(err)
	}
}
