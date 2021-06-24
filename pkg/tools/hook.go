package tools

import (
	"fmt"
	"os"
	"strings"

	"arhat.dev/pkg/exechelper"
	"github.com/fatih/color"

	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/output"
)

type TaskHooks struct {
	field.BaseField

	Before       []Hook `yaml:"before"`
	AfterSuccess []Hook `yaml:"after:success"`
	AfterFailure []Hook `yaml:"after:failure"`
}

type taskExecState uint8

const (
	taskExecBeforeStart taskExecState = iota + 1

	taskExecBeforeMatrixStart
	taskExecAfterMatrixSuccess
	taskExecAfterMatrixFailure

	taskExecAfterSuccess
	taskExecAfterFailure
)

func (h *TaskHooks) Run(
	ctx *field.RenderingContext,
	state taskExecState,
	prefix string,
	prefixColor, outputColor *color.Color,
	allTools map[ToolKey]Tool,
	allShells map[ToolKey]*BaseTool,
) error {
	var toRun []*Hook
	switch state {
	case taskExecBeforeStart:
		for i, bh := range h.Before {
			if bh.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.Before[i])
		}
	case taskExecBeforeMatrixStart:
		for i, bh := range h.Before {
			if !bh.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.Before[i])
		}
	case taskExecAfterMatrixSuccess:
		for i, ash := range h.AfterSuccess {
			if !ash.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.AfterSuccess[i])
		}
	case taskExecAfterMatrixFailure:
		for i, afh := range h.AfterFailure {
			if !afh.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.AfterFailure[i])
		}
	case taskExecAfterSuccess:
		for i, ash := range h.AfterSuccess {
			if ash.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.AfterSuccess[i])
		}
	case taskExecAfterFailure:
		for i, afh := range h.AfterFailure {
			if !afh.isPerMatrixRun() {
				continue
			}

			toRun = append(toRun, &h.AfterFailure[i])
		}
	default:
		return fmt.Errorf("unknonw task exec state: %d", state)
	}

	for _, h := range toRun {
		err := h.Run(ctx, prefix, prefixColor, outputColor, allTools, allShells)
		if err != nil {
			return fmt.Errorf("hook failed: %w", err)
		}
	}

	return nil
}

type Hook struct {
	field.BaseField

	// execute this hook per-matrix run instead of per-task
	PerMatrixRun *bool `yaml:"per_matrix_run"`

	Config map[string]string `dukkha:"other"`
}

func (h *Hook) Run(
	ctx *field.RenderingContext,
	prefix string,
	prefixColor, outputColor *color.Color,
	allTools map[ToolKey]Tool,
	allShells map[ToolKey]*BaseTool,
) error {
	if len(h.Config) == 0 {
		return nil
	}

	if len(h.Config) != 1 {
		return fmt.Errorf("invalid multiple hook kind in one hook")
	}

	var (
		shellKey   *ToolKey
		script     string
		isFilePath bool
	)

	for k, v := range h.Config {
		script = v

		switch {
		case k == "task":
			parts := strings.Split(v, ":")

			var (
				taskKind string
				taskName string
			)

			key := ToolKey{
				ToolKind: parts[0],
				ToolName: "",
			}

			switch len(parts) {
			case 3:
				taskKind = parts[1]
				taskName = parts[2]
			case 4:
				key.ToolName = parts[1]
				taskKind = parts[2]
				taskName = parts[3]
			default:
				return fmt.Errorf("hook: invalid task reference: %q", v)
			}

			tool, ok := allTools[key]
			if !ok {
				return fmt.Errorf("hook: tool %q not found", key.ToolKind+":"+key.ToolName)
			}

			return tool.Run(ctx.Context(), allTools, allShells, taskKind, taskName)
		case strings.HasPrefix(k, "shell_file:"):
			shellKey = &ToolKey{ToolKind: "shell", ToolName: strings.SplitN(k, ":", 2)[1]}
			isFilePath = true
		case k == "shell_file":
			shellKey = &ToolKey{ToolKind: "shell", ToolName: ""}
			isFilePath = true
		case strings.HasPrefix(k, "shell:"):
			shellKey = &ToolKey{ToolKind: "shell", ToolName: strings.SplitN(k, ":", 2)[1]}
			isFilePath = false
		case k == "shell":
			shellKey = &ToolKey{ToolKind: "shell", ToolName: ""}
			isFilePath = false
		default:
			return fmt.Errorf("unknown hook kind: %q", k)
		}
	}

	if shellKey == nil {
		return nil
	}

	sh, ok := allShells[*shellKey]
	if !ok {
		return fmt.Errorf("hook: shell %q not found", shellKey.ToolName)
	}

	scriptCtx := ctx.Clone()
	env, cmd, err := sh.GetExecSpec(script, isFilePath)
	if err != nil {
		return err
	}

	scriptCtx.AddEnv(env...)

	p, err := exechelper.Do(exechelper.Spec{
		Context: scriptCtx.Context(),
		Command: cmd,
		Env:     scriptCtx.Values().Env,

		Stdin:  os.Stdin,
		Stderr: output.PrefixWriter(prefix, prefixColor, outputColor, os.Stderr),
		Stdout: output.PrefixWriter(prefix, prefixColor, outputColor, os.Stdout),
	})
	if err != nil {
		return fmt.Errorf("hook: failed to run script: %w", err)
	}

	code, err := p.Wait()
	if err != nil {
		return fmt.Errorf("hook: command exited with code %d: %w", code, err)
	}

	return nil
}

func (h *Hook) isPerMatrixRun() bool {
	if h.PerMatrixRun == nil {
		// defaults to per-matrix spec
		return true
	}

	return *h.PerMatrixRun
}