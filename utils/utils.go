package utils

import (
	"log"
	"reflect"

	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func IgnoreChanges(ctx *plm.Context, global bool, types []string, props []string) {
	err := ctx.RegisterStackTransformation(
		func(args *plm.ResourceTransformationArgs) *plm.ResourceTransformationResult {
			if global || contains(types, args.Type) {
				return &plm.ResourceTransformationResult{
					Props: args.Props,
					Opts:  append(args.Opts, plm.IgnoreChanges(props)),
				}
			}
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
}

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

func ToPulumiStringArray(a []string) plm.StringArrayInput {
	var res []plm.StringInput
	for _, s := range a {
		res = append(res, plm.String(s))
	}
	return plm.StringArray(res)
}

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}
