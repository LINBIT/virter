package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/cmd"
	"github.com/LINBIT/virter/internal/virter"
)

func TestFileVarReaderUnset(t *testing.T) {
	var f cmd.FileVar
	assert.True(t, f.Reader() == nil, "Reader of an unset FileVar must be an untyped nil")

	pc, err := virter.NewProvisionConfig(f.Reader(), virter.ProvisionOption{
		Overrides: []string{"steps[0].shell.script=true"},
	})
	if assert.NoError(t, err) && assert.Len(t, pc.Steps, 1) {
		assert.Equal(t, "true", pc.Steps[0].Shell.Script)
	}
}

func TestFileVarReaderSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provision.toml")
	err := os.WriteFile(path, []byte("version = 1\n\n[[steps]]\n[steps.shell]\nscript = \"true\"\n"), 0o644)
	assert.NoError(t, err)

	var f cmd.FileVar
	assert.NoError(t, f.Set(path))

	pc, err := virter.NewProvisionConfig(f.Reader(), virter.ProvisionOption{})
	if assert.NoError(t, err) && assert.Len(t, pc.Steps, 1) {
		assert.Equal(t, "true", pc.Steps[0].Shell.Script)
	}
}
