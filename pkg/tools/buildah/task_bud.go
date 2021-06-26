package buildah

import (
	"regexp"
	"strings"

	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/sliceutils"
	"arhat.dev/dukkha/pkg/tools"
)

const TaskKindBud = "bud"

func init() {
	field.RegisterInterfaceField(
		tools.TaskType,
		regexp.MustCompile(`^buildah(:.+){0,1}:bud$`),
		func(params []string) interface{} {
			t := &TaskBud{}
			if len(params) != 0 {
				t.SetToolName(strings.TrimPrefix(params[0], ":"))
			}
			return t
		},
	)
}

var _ tools.Task = (*TaskBud)(nil)

type TaskBud struct {
	field.BaseField

	tools.BaseTask `yaml:",inline"`

	Context    string          `yaml:"context"`
	ImageNames []ImageNameSpec `yaml:"image_names"`
	Dockerfile string          `yaml:"dockerfile"`
	ExtraArgs  []string        `yaml:"extraArgs"`
}

type ImageNameSpec struct {
	Image    string `yaml:"image"`
	Manifest string `yaml:"manifest"`
}

func (c *TaskBud) ToolKind() string { return ToolKind }
func (c *TaskBud) TaskKind() string { return TaskKindBud }

func (c *TaskBud) GetExecSpecs(ctx *field.RenderingContext, toolCmd []string) ([]tools.TaskExecSpec, error) {
	budCmd := sliceutils.NewStringSlice(toolCmd, "bud")
	if len(c.Dockerfile) != 0 {
		budCmd = append(budCmd, "-f", c.Dockerfile)
	}

	// user can override --os/--arch with --platform
	mArch := ctx.Values().Env[constant.ENV_MATRIX_ARCH]
	budCmd = append(budCmd,
		"--os", constant.GetOciOS(ctx.Values().Env[constant.ENV_MATRIX_KERNEL]),
		"--arch", constant.GetOciArch(mArch),
	)

	variant := constant.GetOciArchVariant(mArch)
	if len(variant) != 0 {
		budCmd = append(budCmd, "--variant", variant)
	}

	budCmd = append(budCmd, c.ExtraArgs...)

	targets := c.ImageNames
	if len(targets) == 0 {
		targets = []ImageNameSpec{{
			Image:    c.Name,
			Manifest: "",
		}}
	}

	for _, spec := range targets {
		if len(spec.Image) == 0 {
			continue
		}

		budCmd = append(budCmd, "-t", spec.Image)
	}

	// buildah only allows one --manifest for each bud run

	context := c.Context
	if len(context) == 0 {
		context = "."
	}

	var result []tools.TaskExecSpec
	for _, spec := range targets {
		if len(spec.Manifest) == 0 {
			continue
		}

		result = append(result, tools.TaskExecSpec{
			Command:     sliceutils.NewStringSlice(budCmd, "--manifest", spec.Manifest, context),
			IgnoreError: false,
		})
	}

	if len(result) == 0 {
		// no manifest set
		result = append(result, tools.TaskExecSpec{
			Command:     append(budCmd, context),
			IgnoreError: false,
		})
	}

	return result, nil
}
