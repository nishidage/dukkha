package renderer

import (
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func ToYamlBytes(in interface{}) ([]byte, error) {
	switch t := in.(type) {
	case string:
		return []byte(t), nil
	case []byte:
		return t, nil
	default:
	}

	ret, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func FormatCacheDir(dukkhaCacheDir, rendererName string) string {
	return filepath.Join(dukkhaCacheDir, "renderer-"+rendererName)
}
