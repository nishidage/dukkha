package shell_file

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDriver(t *testing.T) {
	d := New(func(toExec []string, isFilePath bool) (env []string, cmd []string, err error) {
		return
	})
	assert.NotNil(t, d)
}
