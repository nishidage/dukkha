package echo

import (
	"arhat.dev/pkg/yamlhelper"
	"arhat.dev/rs"

	"arhat.dev/dukkha/pkg/dukkha"
)

// nolint:revive
const (
	DefaultName = "echo"
)

func init() { dukkha.RegisterRenderer(DefaultName, NewDefault) }

func NewDefault(name string) dukkha.Renderer { return &driver{name: name} }

var _ dukkha.Renderer = (*driver)(nil)

type driver struct {
	rs.BaseField `yaml:"-"`

	name string
}

func (d *driver) Init(ctx dukkha.ConfigResolvingContext) error { return nil }

func (d *driver) RenderYaml(
	_ dukkha.RenderingContext, rawData interface{},
) ([]byte, error) {
	rawData, err := rs.NormalizeRawData(rawData)
	if err != nil {
		return nil, err
	}

	return yamlhelper.ToYamlBytes(rawData)
}
