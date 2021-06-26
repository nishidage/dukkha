package tools

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/fatih/color"

	"arhat.dev/dukkha/pkg/field"
)

// TaskType for interface type registration
var TaskType = reflect.TypeOf((*Task)(nil)).Elem()

type TaskExecSpec struct {
	Env         []string
	Command     []string
	IgnoreError bool
}

type Task interface {
	field.Interface

	// Kind of the tool managing this task (e.g. docker)
	ToolKind() string

	// Name of the tool managing this task (e.g. my-tool)
	ToolName() string

	// Kind of the task (e.g. build)
	TaskKind() string

	// Name of the task
	TaskName() string

	// GetMatrixSpecs for matrix build
	GetMatrixSpecs(
		ctx *field.RenderingContext,
		rf field.RenderingFunc,
		filter map[string][]string,
	) ([]MatrixSpec, error)

	// GetExecSpecs generate commands using current field values
	GetExecSpecs(ctx *field.RenderingContext, toolCmd []string) ([]TaskExecSpec, error)

	RunHooks(
		ctx *field.RenderingContext,
		rf field.RenderingFunc,
		state taskExecState,
		prefix string,
		prefixColor, outputColor *color.Color,
		allTools map[ToolKey]Tool,
		allShells map[ToolKey]*BaseTool,
	) error
}

type BaseTask struct {
	field.BaseField

	Name   string       `yaml:"name"`
	Matrix MatrixConfig `yaml:"matrix"`
	Hooks  TaskHooks    `yaml:"hooks"`

	toolName string `yaml:"-"`

	hookMU sync.Mutex
}

func (t *BaseTask) RunHooks(
	ctx *field.RenderingContext,
	rf field.RenderingFunc,
	state taskExecState,
	prefix string,
	prefixColor, outputColor *color.Color,
	allTools map[ToolKey]Tool,
	allShells map[ToolKey]*BaseTool,
) error {
	t.hookMU.Lock()
	defer t.hookMU.Unlock()

	err := t.Hooks.ResolveFields(ctx, rf, -1)
	if err != nil {
		return fmt.Errorf("failed to resolve hooks field: %w", err)
	}

	return t.Hooks.Run(ctx, state, prefix, prefixColor, outputColor, allTools, allShells)
}

func (t *BaseTask) ToolName() string        { return t.toolName }
func (t *BaseTask) SetToolName(name string) { t.toolName = name }
func (t *BaseTask) TaskName() string        { return t.Name }

func (t *BaseTask) GetMatrixSpecs(
	ctx *field.RenderingContext,
	rf field.RenderingFunc,
	filter map[string][]string,
) ([]MatrixSpec, error) {
	// resolve matrix config first
	err := t.ResolveFields(ctx, rf, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base task fields: %w", err)
	}

	err = t.Matrix.ResolveFields(ctx, rf, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task matrix: %w", err)
	}

	return t.Matrix.GetSpecs(filter), nil
}
