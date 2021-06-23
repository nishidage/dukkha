package field

import (
	"context"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Interface interface {
	yaml.Unmarshaler

	// ResolveFields resolves struct fields with rendering suffix
	ResolveFields(ctx *RenderingContext, render RenderingFunc, depth int) error
}

type ExecSpecGetFunc func(script string, isFilePath bool) (env, cmd []string, err error)

type (
	RenderingFunc func(ctx *RenderingContext, renderer, rawData string) (string, error)

	RenderingValues struct {
		Env map[string]string
	}

	RenderingContext struct {
		ctx    context.Context
		values *RenderingValues
	}
)

func WithRenderingValues(ctx context.Context, env []string) *RenderingContext {
	ret := &RenderingContext{
		ctx: ctx,
		values: &RenderingValues{
			Env: make(map[string]string),
		},
	}

	ret.AddEnv(os.Environ()...)
	ret.AddEnv(env...)

	return ret
}

func (c *RenderingContext) Context() context.Context {
	return c.ctx
}

func (c *RenderingContext) AddEnv(entries ...string) {
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		key, value := parts[0], ""
		if len(parts) == 2 {
			value = parts[1]
		}

		// do not expand environment variables
		c.values.Env[key] = value
	}
}

func (c *RenderingContext) Clone() *RenderingContext {
	ret := &RenderingContext{
		ctx: c.ctx,
		values: &RenderingValues{
			Env: make(map[string]string),
		},
	}

	for k, v := range c.values.Env {
		ret.values.Env[k] = v
	}

	return ret
}

func (c *RenderingContext) Values() *RenderingValues {
	return c.values
}
