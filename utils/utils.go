package utils

import (
	"log"
	"reflect"

	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func RegisterAutoTags(ctx *plm.Context, autoTags plm.StringMap) {
	err := ctx.RegisterStackTransformation(
		func(args *plm.ResourceTransformationArgs) *plm.ResourceTransformationResult {
			ptr := reflect.ValueOf(args.Props)
			if ptr.IsValid() {
				val := ptr.Elem()
				tags := val.FieldByName("Tags")

				if tags.IsValid() {

					var tagsMap plm.StringMap
					if !tags.IsZero() {
						tagsMap = tags.Interface().(plm.StringMap)
					} else {
						tagsMap = map[string]plm.StringInput{}
					}
					for k, v := range autoTags {
						tagsMap[k] = v
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
