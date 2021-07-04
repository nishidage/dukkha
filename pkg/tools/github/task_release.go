package github

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/sliceutils"
	"arhat.dev/dukkha/pkg/tools"
)

const TaskKindRelease = "release"

func init() {
	field.RegisterInterfaceField(
		tools.TaskType,
		regexp.MustCompile(`^github(:.+)?:release$`),
		func(subMatches []string) interface{} {
			t := &TaskRelease{}
			if len(subMatches) != 0 {
				t.SetToolName(strings.TrimPrefix(subMatches[0], ":"))
			}
			return t
		},
	)
}

var _ tools.Task = (*TaskRelease)(nil)

type TaskRelease struct {
	field.BaseField

	tools.BaseTask `yaml:",inline"`

	Tag        string `yaml:"tag"`
	Draft      bool   `yaml:"draft"`
	PreRelease bool   `yaml:"pre_release"`
	Title      string `yaml:"title"`
	Notes      string `yaml:"notes"`

	Files []ReleaseFileSpec `yaml:"files"`
}

type ReleaseFileSpec struct {
	// path to the file, can use glob
	Path string `yaml:"path"`
	// the display label as noted in gh docs
	// https://cli.github.com/manual/gh_release_create
	Label string `yaml:"label"`
}

func (c *TaskRelease) ToolKind() string { return ToolKind }
func (c *TaskRelease) TaskKind() string { return TaskKindRelease }

func (c *TaskRelease) GetExecSpecs(ctx *field.RenderingContext, ghCmd []string) ([]tools.TaskExecSpec, error) {
	createCmd := sliceutils.NewStringSlice(
		ghCmd, "release", "create", c.Tag,
	)

	if c.Draft {
		createCmd = append(createCmd, "--draft")
	}

	if c.PreRelease {
		createCmd = append(createCmd, "--prerelease")
	}

	if len(c.Title) != 0 {
		createCmd = append(createCmd,
			"--title", fmt.Sprintf("%q", c.Title),
		)
	}

	if len(c.Notes) != 0 {
		cacheDir := ctx.Values().Env[constant.ENV_DUKKHA_CACHE_DIR]
		f, err := ioutil.TempFile(cacheDir, "github-release-note-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary release note file: %w", err)
		}

		noteFile := f.Name()
		_, err = f.Write([]byte(c.Notes))
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to write release note: %w", err)
		}

		createCmd = append(createCmd, "--notes-file", noteFile)
	}

	for _, spec := range c.Files {
		matches, err := filepath.Glob(spec.Path)
		if err != nil {
			matches = []string{spec.Path}
		}

		for i, file := range matches {
			var arg string
			if len(spec.Label) != 0 {
				arg = `'` + file + `#` + spec.Label
				if i != 0 {
					arg += " " + strconv.FormatInt(int64(i), 10)
				}

				arg += `'`
			} else {
				arg = `'` + file + `#` + filepath.Base(file) + `'`
			}

			createCmd = append(createCmd, arg)
		}
	}

	return []tools.TaskExecSpec{{
		Command: createCmd,
	}}, nil
}