package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellValue_UnmarshalYAML(t *testing.T) {
	t.Run("string without args", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`bash`), &s)
		require.NoError(t, err)
		assert.False(t, s.IsZero())
		assert.Equal(t, "bash", s.Command())
		assert.Empty(t, s.Arguments())
		assert.False(t, s.IsArray())
	})

	t.Run("string with args inline", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`"bash -e"`), &s)
		require.NoError(t, err)
		assert.Equal(t, "bash -e", s.Command())
		assert.Empty(t, s.Arguments())
	})

	t.Run("array form inline", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`["bash", "-e", "-x"]`), &s)
		require.NoError(t, err)
		assert.Equal(t, "bash", s.Command())
		assert.Equal(t, []string{"-e", "-x"}, s.Arguments())
		assert.True(t, s.IsArray())
	})

	t.Run("multiline array form", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte("- bash\n- -e\n- -x"), &s)
		require.NoError(t, err)
		assert.Equal(t, "bash", s.Command())
		assert.Equal(t, []string{"-e", "-x"}, s.Arguments())
	})

	t.Run("empty string", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`""`), &s)
		require.NoError(t, err)
		assert.False(t, s.IsZero()) // Was set, just empty
		assert.Equal(t, "", s.Command())
	})

	t.Run("empty array", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`[]`), &s)
		require.NoError(t, err)
		assert.Equal(t, "", s.Command())
		assert.Empty(t, s.Arguments())
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var s types.ShellValue
		assert.True(t, s.IsZero())
	})

	t.Run("invalid type map", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`{key: value}`), &s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or array")
	})

	t.Run("shell with environment variable syntax", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`"${SHELL}"`), &s)
		require.NoError(t, err)
		assert.Equal(t, "${SHELL}", s.Command())
	})

	t.Run("nix-shell example", func(t *testing.T) {
		var s types.ShellValue
		err := yaml.Unmarshal([]byte(`["nix-shell", "-p", "python3"]`), &s)
		require.NoError(t, err)
		assert.Equal(t, "nix-shell", s.Command())
		assert.Equal(t, []string{"-p", "python3"}, s.Arguments())
	})
}

func TestShellValue_InStruct(t *testing.T) {
	type Config struct {
		Shell types.ShellValue `yaml:"shell"`
		Name  string           `yaml:"name"`
	}

	t.Run("shell set", func(t *testing.T) {
		data := `
name: test
shell: bash
`
		var cfg Config
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "test", cfg.Name)
		assert.Equal(t, "bash", cfg.Shell.Command())
		assert.False(t, cfg.Shell.IsZero())
	})

	t.Run("shell not set", func(t *testing.T) {
		data := `name: test`
		var cfg Config
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Shell.IsZero())
	})
}
