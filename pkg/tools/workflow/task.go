package workflow

import (
	"fmt"
	"io"
	"regexp"

	"arhat.dev/dukkha/pkg/dukkha"
	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/tools"
	"arhat.dev/dukkha/pkg/types"
)

const TaskKindRun = "run"

func init() {
	field.RegisterInterfaceField(
		dukkha.TaskType,
		regexp.MustCompile("^workflow:run$"),
		func(subMatches []string) interface{} {
			return &TaskRun{}
		},
	)
}

var _ dukkha.Task = (*TaskRun)(nil)

type TaskRun struct {
	field.BaseField

	tools.BaseTask

	Jobs []tools.Hook `yaml:"jobs"`
}

func (w *TaskRun) ToolKind() dukkha.ToolKind { return ToolKind }
func (w *TaskRun) Kind() dukkha.TaskKind     { return TaskKindRun }

func (w *TaskRun) GetExecSpecs(rc types.RenderingContext, _ []string) ([]dukkha.TaskExecSpec, error) {
	var ret []dukkha.TaskExecSpec
	for i, job := range w.Jobs {
		if len(job.Task) != 0 {
			// do task
			ref, err := dukkha.ParseTaskReference(job.Task)
			if err != nil {
				return nil, fmt.Errorf("invalid task reference at job#%d: %w", i, err)
			}

			_ = ref

			// TODO: deep copy current job, execute as a hook
			ret = append(ret, dukkha.TaskExecSpec{
				AlterExecFunc: func(
					replace map[string][]byte,
					stdin io.Reader,
					stdout, stderr io.Writer,
				) ([]dukkha.TaskExecSpec, error) {
					// TODO: implement
					return nil, nil
				},
			})
			continue
		}

		// run shell command
		ret = append(ret, dukkha.TaskExecSpec{
			Command: []string{},
		})
	}

	return ret, nil
}
