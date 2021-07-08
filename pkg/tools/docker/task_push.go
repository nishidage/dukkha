package docker

import (
	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/dukkha"
	"arhat.dev/dukkha/pkg/sliceutils"
	"arhat.dev/dukkha/pkg/tools/buildah"
)

const TaskKindPush = "push"

func init() {
	dukkha.RegisterTask(
		ToolKind, TaskKindPush,
		func(toolName string) dukkha.Task {
			t := &TaskPush{}
			t.SetToolName(toolName)
			return t
		},
	)
}

var _ dukkha.Task = (*TaskPush)(nil)

type TaskPush buildah.TaskPush

func (c *TaskPush) ToolKind() dukkha.ToolKind { return ToolKind }
func (c *TaskPush) Kind() dukkha.TaskKind     { return TaskKindPush }

func (c *TaskPush) GetExecSpecs(rc dukkha.RenderingContext, toolCmd []string) ([]dukkha.TaskExecSpec, error) {
	targets := c.ImageNames
	if len(targets) == 0 {
		targets = []buildah.ImageNameSpec{{
			Image:    c.TaskName,
			Manifest: "",
		}}
	}

	var (
		result []dukkha.TaskExecSpec

		manifestCmd = sliceutils.NewStrings(toolCmd, "manifest")
	)

	for _, spec := range targets {
		if len(spec.Image) == 0 {
			continue
		}

		imageName := buildah.SetDefaultImageTagIfNoTagSet(rc, spec.Image)
		// docker push <image-name>
		if buildah.ImageOrManifestHasFQDN(imageName) {
			result = append(result, dukkha.TaskExecSpec{
				Command: sliceutils.NewStrings(
					toolCmd, "push", imageName,
				),
				IgnoreError: false,
			})
		}

		if len(spec.Manifest) == 0 {
			continue
		}

		manifestName := buildah.SetDefaultManifestTagIfNoTagSet(rc, spec.Manifest)
		result = append(result,
			// ensure manifest exists
			dukkha.TaskExecSpec{
				Command: sliceutils.NewStrings(
					manifestCmd, "create", manifestName, imageName,
				),
				// may already exists
				IgnoreError: true,
			},
			// link manifest and image
			dukkha.TaskExecSpec{
				Command: sliceutils.NewStrings(
					manifestCmd, "create", manifestName,
					"--amend", imageName,
				),
				IgnoreError: false,
			},
		)

		// docker manifest annotate \
		// 		<manifest-list-name> <image-name> \
		// 		--os <arch> --arch <arch> {--variant <variant>}
		mArch := rc.MatrixArch()
		annotateCmd := sliceutils.NewStrings(
			manifestCmd, "annotate", manifestName, imageName,
			"--os", constant.GetDockerOS(rc.MatrixKernel()),
			"--arch", constant.GetDockerArch(mArch),
		)

		variant := constant.GetDockerArchVariant(mArch)
		if len(variant) != 0 {
			annotateCmd = append(annotateCmd, "--variant", variant)
		}

		result = append(result, dukkha.TaskExecSpec{
			Command:     annotateCmd,
			IgnoreError: false,
		})

		// docker manifest push <manifest-list-name>
		if buildah.ImageOrManifestHasFQDN(manifestName) {
			result = append(result, dukkha.TaskExecSpec{
				Command:     sliceutils.NewStrings(toolCmd, "manifest", "push", spec.Manifest),
				IgnoreError: false,
			})
		}
	}

	return result, nil
}
