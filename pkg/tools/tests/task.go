package tests

import (
	"context"
	"reflect"
	"testing"

	"arhat.dev/pkg/testhelper"
	"arhat.dev/rs"
	"github.com/stretchr/testify/assert"

	di "arhat.dev/dukkha/internal"
	"arhat.dev/dukkha/pkg/dukkha"
	dt "arhat.dev/dukkha/pkg/dukkha/test"
	"arhat.dev/dukkha/pkg/renderer/af"
	"arhat.dev/dukkha/pkg/renderer/env"
	"arhat.dev/dukkha/pkg/renderer/file"
	"arhat.dev/dukkha/pkg/renderer/shell"
	"arhat.dev/dukkha/pkg/renderer/tpl"
	"arhat.dev/dukkha/pkg/tools"
)

type ExecSpecGenerationTestCase struct {
	Name     string
	Prepare  func() error
	Finalize func()

	Options   dukkha.TaskMatrixExecOptions
	Task      dukkha.Task
	Expected  []dukkha.TaskExecSpec
	ExpectErr bool
}

func RunTaskExecSpecGenerationTests(
	t *testing.T,
	taskCtx dukkha.TaskExecContext,
	tests []ExecSpecGenerationTestCase,
) {
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			runTaskTest(taskCtx, &test, t)
		})
	}
}

type baseTaskInitializer interface {
	InitBaseTask(
		k dukkha.ToolKind,
		n dukkha.ToolName,
		tk dukkha.TaskKind,
		impl dukkha.Task,
	)
}

func runTaskTest(taskCtx dukkha.TaskExecContext, test *ExecSpecGenerationTestCase, t *testing.T) {
	if test.Finalize != nil {
		defer test.Finalize()
	}

	if test.Prepare != nil {
		if !assert.NoError(t, test.Prepare(), "failed to prepare test environment") {
			return
		}
	}

	rs.InitRecursively(reflect.ValueOf(test.Task), nil)

	// nolint:gocritic
	switch t := test.Task.(type) {
	case baseTaskInitializer:
		t.InitBaseTask(test.Task.ToolKind(), test.Task.ToolName(), test.Task.Kind(), test.Task)
	}

	if test.ExpectErr {
		_, err := test.Task.GetExecSpecs(taskCtx, test.Options)
		assert.Error(t, err)
		return
	}

	specs, err := test.Task.GetExecSpecs(taskCtx, test.Options)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, test.Expected, specs)
}

func TestFixturesUsingRenderingSuffix(
	t *testing.T,
	dir string,
	rc rs.RenderingHandler,
	newTestSpec func() rs.Field,
	newCheckSpec func() rs.Field,
	check func(t *testing.T, ts, cs rs.Field),
) {
	testhelper.TestFixtures(t, dir,
		func() interface{} { return rs.Init(newTestSpec(), nil) },
		func() interface{} { return rs.Init(newCheckSpec(), nil) },
		func(t *testing.T, spec, exp interface{}) {
			defer t.Cleanup(func() {})
			s, e := spec.(rs.Field), exp.(rs.Field)

			ctx := dt.NewTestContext(context.TODO())
			ctx.(di.CacheDirSetter).SetCacheDir(t.TempDir())
			ctx.AddRenderer("file", file.NewDefault("file"))
			ctx.AddRenderer("env", env.NewDefault("env"))
			ctx.AddRenderer("tpl", tpl.NewDefault("tpl"))
			ctx.AddRenderer("shell", shell.NewDefault("shell"))

			afr := af.NewDefault("af")
			assert.NoError(t, afr.Init(ctx))
			ctx.AddRenderer("af", afr)

			assert.NoError(t, s.ResolveFields(rc, -1))
			assert.NoError(t, e.ResolveFields(rc, -1))

			check(t, s, e)
		},
	)
}

func TestTask(
	t *testing.T,
	dir string,
	tool dukkha.Tool,
	newTask func() dukkha.Task,
	newExpected func() rs.Field,
	check func(t *testing.T, expected, actual rs.Field),
) {
	type TestCase struct {
		rs.BaseField

		Env dukkha.Env `yaml:"env"`

		// Tool dukkha.Tool `yaml:"tool"`
		Task dukkha.Task `yaml:"task"`
	}

	type CheckSpec struct {
		rs.BaseField

		ExpectErr bool     `yaml:"expect_err"`
		Actual    rs.Field `yaml:"actual"`
		Expected  rs.Field `yaml:"expected"`
	}

	testhelper.TestFixtures(t, dir,
		func() interface{} {
			return rs.Init(&TestCase{}, &rs.Options{
				InterfaceTypeHandler: rs.InterfaceTypeHandleFunc(
					func(typ reflect.Type, yamlKey string) (interface{}, error) {
						return rs.Init(newTask(), nil), nil
					},
				),
			})
		},
		func() interface{} {
			return rs.Init(&CheckSpec{}, &rs.Options{
				InterfaceTypeHandler: rs.InterfaceTypeHandleFunc(
					func(typ reflect.Type, yamlKey string) (interface{}, error) {
						return rs.Init(newExpected(), nil), nil
					},
				),
			})
		},
		func(t *testing.T, in, exp interface{}) {
			defer t.Cleanup(func() {

			})
			spec := in.(*TestCase)
			e := exp.(*CheckSpec)

			ctx := dt.NewTestContext(context.TODO())
			ctx.(di.CacheDirSetter).SetCacheDir(t.TempDir())
			ctx.AddRenderer("file", file.NewDefault("file"))
			ctx.AddRenderer("env", env.NewDefault("env"))
			ctx.AddRenderer("tpl", tpl.NewDefault("tpl"))
			ctx.AddRenderer("shell", shell.NewDefault("shell"))

			afr := af.NewDefault("af")
			assert.NoError(t, afr.Init(ctx))
			ctx.AddRenderer("af", afr)

			if !assert.NoError(t, dukkha.ResolveEnv(ctx, spec, "Env", "env")) {
				return
			}

			if !assert.NoError(t, spec.ResolveFields(ctx, -1)) {
				return
			}

			rs.Init(tool, nil)

			assert.NoError(t, tool.Init("", ctx.CacheDir()))
			ctx.AddTool(tool.Key(), tool)

			assert.NoError(t, tool.AddTasks([]dukkha.Task{spec.Task}))

			err := tools.RunTask(&tools.TaskExecRequest{
				Context:     ctx,
				Tool:        tool,
				Task:        spec.Task,
				IgnoreError: false,
			})

			if !assert.NoError(t, e.ResolveFields(ctx, -1)) {
				return
			}

			if e.ExpectErr {
				assert.Error(t, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			check(t, e.Expected, e.Actual)
		},
	)
}
