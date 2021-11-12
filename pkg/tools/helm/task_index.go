package helm

import (
	"fmt"
	"path/filepath"

	"arhat.dev/rs"

	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/dukkha"
	"arhat.dev/dukkha/pkg/tools"
)

const TaskKindIndex = "index"

func init() {
	dukkha.RegisterTask(
		ToolKind, TaskKindIndex,
		func(toolName string) dukkha.Task {
			t := &TaskIndex{}
			t.InitBaseTask(ToolKind, dukkha.ToolName(toolName), TaskKindIndex, t)
			return t
		},
	)
}

type TaskIndex struct {
	rs.BaseField `yaml:"-"`

	tools.BaseTask `yaml:",inline"`

	RepoURL     string `yaml:"repo_url"`
	PackagesDir string `yaml:"packages_dir"`
	Merge       string `yaml:"merge"`
}

func (c *TaskIndex) GetExecSpecs(
	rc dukkha.TaskExecContext, options dukkha.TaskMatrixExecOptions,
) ([]dukkha.TaskExecSpec, error) {

	indexCmd := []string{constant.DUKKHA_TOOL_CMD, "repo", "index"}

	err := c.DoAfterFieldsResolved(rc, -1, true, func() error {
		if len(c.RepoURL) != 0 {
			indexCmd = append(indexCmd, "--url", c.RepoURL)
		}

		dukkhaWorkingDir := rc.WorkingDir()
		if len(c.PackagesDir) != 0 {
			pkgDir, err := filepath.Abs(c.PackagesDir)
			if err != nil {
				return fmt.Errorf(
					"failed to determine absolute path of package_dir: %w",
					err,
				)
			}

			indexCmd = append(indexCmd, pkgDir)
		} else {
			indexCmd = append(indexCmd, dukkhaWorkingDir)
		}

		if len(c.Merge) != 0 {
			indexCmd = append(indexCmd, "--merge", c.Merge)
		}

		return nil
	})

	return []dukkha.TaskExecSpec{{
		Command:   indexCmd,
		UseShell:  options.UseShell(),
		ShellName: options.ShellName(),
	}}, err
}
