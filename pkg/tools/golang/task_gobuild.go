package golang

import (
	"fmt"
	"strings"

	"arhat.dev/dukkha/pkg/constant"
	"arhat.dev/dukkha/pkg/dukkha"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/sliceutils"
	"arhat.dev/dukkha/pkg/tools"
)

const TaskKindBuild = "build"

func init() {
	dukkha.RegisterTask(
		ToolKind, TaskKindBuild,
		func(toolName string) dukkha.Task {
			t := &TaskBuild{}
			t.InitBaseTask(ToolKind, dukkha.ToolName(toolName), TaskKindBuild, t)
			return t
		},
	)
}

type TaskBuild struct {
	field.BaseField

	tools.BaseTask `yaml:",inline"`

	Chdir     string   `yaml:"chdir"`
	Path      string   `yaml:"path"`
	LDFlags   []string `yaml:"ldflags"`
	Tags      []string `yaml:"tags"`
	ExtraArgs []string `yaml:"extra_args"`
	Outputs   []string `yaml:"outputs"`

	CGO CGOSepc `yaml:"cgo"`
}

type CGOSepc struct {
	field.BaseField

	Enabled bool     `yaml:"enabled"`
	CFlags  []string `yaml:"cflags"`
	LDFlags []string `yaml:"ldflags"`

	CC  string `yaml:"cc"`
	CXX string `yaml:"cxx"`
}

func (c *TaskBuild) GetExecSpecs(
	rc dukkha.TaskExecContext, options *dukkha.TaskExecOptions,
) ([]dukkha.TaskExecSpec, error) {
	mKernel := rc.MatrixKernel()
	mArch := rc.MatrixArch()
	doingCrossCompiling := rc.HostKernel() != mKernel ||
		rc.HostArch() != mArch

	env := sliceutils.NewStrings(
		c.CGO.getEnv(options.UseShell,
			doingCrossCompiling, mKernel, mArch,
			rc.HostOS(),
			rc.MatrixLibc(),
		),
	)

	env = append(env, "GOOS="+constant.GetGolangOS(mKernel))
	env = append(env, "GOARCH="+constant.GetGolangArch(mArch))

	if envGOMIPS := c.getGOMIPS(mArch); len(envGOMIPS) != 0 {
		env = append(env, "GOMIPS="+envGOMIPS, "GOMIPS64="+envGOMIPS)
	}

	if envGOARM := c.getGOARM(mArch); len(envGOARM) != 0 {
		env = append(env, "GOARM="+envGOARM)
	}

	var buildSteps []dukkha.TaskExecSpec

	err := c.DoAfterFieldsResolved(rc, -1, func() error {
		outputs := c.Outputs
		if len(outputs) == 0 {
			outputs = []string{c.BaseTask.TaskName}
		}

		for _, output := range outputs {
			spec := &dukkha.TaskExecSpec{
				Chdir: c.Chdir,

				// put generated env first, so user can override them
				Env:       sliceutils.NewStrings(env, c.Env...),
				Command:   sliceutils.NewStrings(options.ToolCmd, "build", "-o", output),
				UseShell:  options.UseShell,
				ShellName: options.ShellName,
			}

			spec.Command = append(spec.Command, c.ExtraArgs...)

			if len(c.LDFlags) != 0 {
				spec.Command = append(
					spec.Command,
					"-ldflags",
					formatArgs(c.LDFlags, options.UseShell),
				)
			}

			if len(c.Tags) != 0 {
				spec.Command = append(
					spec.Command,
					"-tags",
					// ref: https://golang.org/doc/go1.13#go-command
					// The go build flag -tags now takes a comma-separated list of build tags,
					// to allow for multiple tags in GOFLAGS. The space-separated form is
					// deprecated but still recognized and will be maintained.
					strings.Join(c.Tags, ","),
				)
			}

			if len(c.Path) != 0 {
				spec.Command = append(spec.Command, c.Path)
			} else {
				spec.Command = append(spec.Command, "./")
			}

			buildSteps = append(buildSteps, *spec)
		}
		return nil
	})

	return buildSteps, err
}

func (c *TaskBuild) getGOARM(mArch string) string {
	if strings.HasPrefix(mArch, "armv") {
		return strings.TrimPrefix(mArch, "armv")
	}

	return ""
}

func (c *TaskBuild) getGOMIPS(mArch string) string {
	if !strings.HasPrefix(mArch, "mips") {
		return ""
	}

	if strings.HasSuffix(mArch, "hf") {
		return "hardfloat"
	}

	return "softfloat"
}

func (c *CGOSepc) getEnv(useShell, doingCrossCompiling bool, mKernel, mArch, hostOS, targetLibc string) []string {
	if !c.Enabled {
		return []string{"CGO_ENABLED=0"}
	}

	var ret []string
	ret = append(ret, "CGO_ENABLED=1")

	if len(c.CFlags) != 0 {
		ret = append(ret,
			fmt.Sprintf("CGO_CFLAGS=%s", formatArgs(c.CFlags, useShell)),
		)
	}

	if len(c.LDFlags) != 0 {
		ret = append(ret,
			fmt.Sprintf("CGO_LDFLAGS=%s", formatArgs(c.LDFlags, useShell)),
		)
	}

	cc := "gcc"
	cxx := "g++"
	if doingCrossCompiling {
		switch hostOS {
		case constant.OS_DEBIAN,
			constant.OS_UBUNTU:
			var tripleName string
			switch mKernel {
			case constant.KERNEL_LINUX:
				tripleName = constant.GetDebianTripleName(mArch, mKernel, targetLibc)
			case constant.KERNEL_DARWIN:
				// TODO: set darwin version
				tripleName = constant.GetAppleTripleName(mArch, "")
			case constant.KERNEL_WINDOWS:
				tripleName = constant.GetDebianTripleName(mArch, mKernel, targetLibc)
			default:
			}

			cc = tripleName + "-gcc"
			cxx = tripleName + "-g++"
		case constant.OS_ALPINE:
			tripleName := constant.GetAlpineTripleName(mArch)
			cc = tripleName + "-gcc"
			cxx = tripleName + "-g++"
		case constant.OS_MACOS:
			cc = "clang"
			cxx = "clang++"
		}
	}

	if len(c.CC) != 0 {
		ret = append(ret, "CC="+c.CC)
	} else if doingCrossCompiling {
		ret = append(ret, "CC="+cc)
	}

	if len(c.CXX) != 0 {
		ret = append(ret, "CXX="+c.CC)
	} else if doingCrossCompiling {
		ret = append(ret, "CXX="+cxx)
	}

	return ret
}

func formatArgs(args []string, useShell bool) string {
	ret := strings.Join(args, " ")
	if useShell {
		ret = `"` + ret + `"`
	}

	return ret
}
