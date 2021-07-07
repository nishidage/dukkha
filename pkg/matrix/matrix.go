package matrix

import (
	"os"

	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/types"
)

type Spec struct {
	field.BaseField

	Include []map[string][]string `yaml:"include"`
	Exclude []map[string][]string `yaml:"exclude"`

	Kernel []string `yaml:"kernel"`
	Arch   []string `yaml:"arch"`

	// catch other matrix fields
	Custom map[string][]string `dukkha:"other"`
}

func (mc *Spec) GetSpecs(matchFilter map[string][]string) []types.MatrixSpec {
	if mc == nil {
		return []types.MatrixSpec{
			{
				"kernel": os.Getenv(constant.ENV_HOST_KERNEL),
				"arch":   os.Getenv(constant.ENV_HOST_ARCH),
			},
		}
	}

	hasUserValue := len(mc.Include) != 0 || len(mc.Exclude) != 0
	hasUserValue = hasUserValue || len(mc.Kernel) != 0 || len(mc.Arch) != 0 || len(mc.Custom) != 0

	if !hasUserValue {
		return []types.MatrixSpec{
			{
				"kernel": os.Getenv(constant.ENV_HOST_KERNEL),
				"arch":   os.Getenv(constant.ENV_HOST_ARCH),
			},
		}
	}

	all := make(map[string][]string)

	if len(mc.Kernel) != 0 {
		all["kernel"] = mc.Kernel
	}

	if len(mc.Arch) != 0 {
		all["arch"] = mc.Arch
	}

	for name := range mc.Custom {
		all[name] = mc.Custom[name]
	}

	// remove excluded
	var removeMatchList []map[string]string
	for _, ex := range mc.Exclude {
		removeMatchList = append(removeMatchList, CartesianProduct(ex)...)
	}

	var result []types.MatrixSpec

	var mf []map[string]string
	if len(matchFilter) != 0 {
		mf = CartesianProduct(matchFilter)
	}
	mat := CartesianProduct(all)
loop:
	for i := range mat {
		spec := types.MatrixSpec(mat[i])

		for _, toRemove := range removeMatchList {
			if spec.Match(toRemove) {
				continue loop
			}
		}

		if len(mf) == 0 {
			// no filter, add it
			result = append(result, spec)
			continue
		}

		for _, f := range mf {
			if spec.Match(f) {
				result = append(result, spec)
				continue loop
			}
		}
	}

	// add included
	for _, inc := range mc.Include {
		mat := CartesianProduct(inc)
	addInclude:
		for i := range mat {
			includeSpec := types.MatrixSpec(mat[i])

			for _, spec := range result {
				if spec.Equals(includeSpec) {
					continue addInclude
				}
			}

			if len(mf) == 0 {
				// no filter, add it
				result = append(result, includeSpec)
				continue
			}

			for _, f := range mf {
				if includeSpec.Match(f) {
					result = append(result, includeSpec)
					continue addInclude
				}
			}
		}
	}

	return result
}
