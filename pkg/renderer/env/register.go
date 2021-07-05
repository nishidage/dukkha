package env

import (
	"fmt"

	"arhat.dev/dukkha/pkg/renderer"
)

func init() {
	renderer.Register(&Config{}, NewDriver)
}

func NewDriver(config interface{}) (renderer.Interface, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unexpected non %s renderer config: %T", DefaultName, config)
	}

	if cfg.GetExecSpec == nil {
		return nil, fmt.Errorf("required GetExecSpec func not set")
	}

	return &Driver{getExecSpec: cfg.GetExecSpec}, nil
}
