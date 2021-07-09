package buildah

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"arhat.dev/dukkha/pkg/dukkha"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/sliceutils"
	"arhat.dev/dukkha/pkg/tools"
)

const TaskKindPush = "push"

func init() {
	dukkha.RegisterTask(
		ToolKind, TaskKindPush,
		func(toolName string) dukkha.Task {
			t := &TaskPush{}
			t.InitBaseTask(ToolKind, dukkha.ToolName(toolName), TaskKindPush)
			return t
		},
	)
}

type TaskPush struct {
	field.BaseField

	tools.BaseTask `yaml:",inline"`

	ImageNames []ImageNameSpec `yaml:"image_names"`
}

func (c *TaskPush) GetExecSpecs(
	rc dukkha.TaskExecContext,
	useShell bool,
	shellName string,
	buildahCmd []string,
) ([]dukkha.TaskExecSpec, error) {
	targets := c.ImageNames
	if len(targets) == 0 {
		targets = []ImageNameSpec{
			{
				Image:    c.TaskName,
				Manifest: "",
			},
		}
	}

	dukkhaCacheDir := rc.CacheDir()

	var result []dukkha.TaskExecSpec
	for _, spec := range targets {
		if len(spec.Image) != 0 {
			imageName := SetDefaultImageTagIfNoTagSet(rc, spec.Image)
			imageIDFile := GetImageIDFileForImageName(
				dukkhaCacheDir, imageName,
			)
			imageIDBytes, err := os.ReadFile(imageIDFile)
			if err != nil {
				return nil, fmt.Errorf("image id file not found: %w", err)
			}

			result = append(result, dukkha.TaskExecSpec{
				Command: sliceutils.NewStrings(
					buildahCmd, "push",
					string(bytes.TrimSpace(imageIDBytes)),
					// TODO: support other destination
					"docker://"+imageName,
				),
				IgnoreError: false,
				UseShell:    useShell,
				ShellName:   shellName,
			})
		}

		if len(spec.Manifest) == 0 {
			continue
		}

		// buildah manifest push --all \
		//   <manifest-list-name> <transport>:<transport-details>
		manifestName := SetDefaultManifestTagIfNoTagSet(rc, spec.Manifest)
		result = append(result, dukkha.TaskExecSpec{
			Command: sliceutils.NewStrings(
				buildahCmd, "manifest", "push", "--all",
				getLocalManifestName(manifestName),
				// TODO: support other destination
				"docker://"+manifestName,
			),
			IgnoreError: false,
			UseShell:    useShell,
			ShellName:   shellName,
		})
	}

	return result, nil
}

func ImageOrManifestHasFQDN(s string) bool {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 1 {
		return false
	}

	return strings.Contains(parts[0], ".")
}
