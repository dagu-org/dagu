package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortValue_UnmarshalYAML(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`22`), &p)
		require.NoError(t, err)
		assert.Equal(t, "22", p.String())
		assert.False(t, p.IsZero())
	})

	t.Run("string", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`"8080"`), &p)
		require.NoError(t, err)
		assert.Equal(t, "8080", p.String())
	})

	t.Run("large port number", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`65535`), &p)
		require.NoError(t, err)
		assert.Equal(t, "65535", p.String())
	})

	t.Run("not set - zero value", func(t *testing.T) {
		var p types.PortValue
		assert.True(t, p.IsZero())
		assert.Equal(t, "", p.String())
	})

	t.Run("invalid type - array", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`[22, 80]`), &p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or number")
	})

	t.Run("invalid type - map", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`{port: 22}`), &p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be string or number")
	})

	t.Run("float with decimal - must be integer", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`22.5`), &p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port must be an integer")
	})

	t.Run("float whole number - allowed", func(t *testing.T) {
		var p types.PortValue
		err := yaml.Unmarshal([]byte(`22.0`), &p)
		require.NoError(t, err)
		assert.Equal(t, "22", p.String())
	})
}

func TestPortValue_InStruct(t *testing.T) {
	type SSHConfig struct {
		Host string          `yaml:"host"`
		Port types.PortValue `yaml:"port"`
		User string          `yaml:"user"`
	}

	t.Run("port as integer", func(t *testing.T) {
		data := `
host: example.com
port: 22
user: admin
`
		var cfg SSHConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "example.com", cfg.Host)
		assert.Equal(t, "22", cfg.Port.String())
		assert.Equal(t, "admin", cfg.User)
	})

	t.Run("port as string", func(t *testing.T) {
		data := `
host: example.com
port: "2222"
user: admin
`
		var cfg SSHConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "2222", cfg.Port.String())
	})

	t.Run("port not set", func(t *testing.T) {
		data := `
host: example.com
user: admin
`
		var cfg SSHConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Port.IsZero())
	})

	type SMTPConfig struct {
		Host     string          `yaml:"host"`
		Port     types.PortValue `yaml:"port"`
		Username string          `yaml:"username"`
	}

	t.Run("smtp port", func(t *testing.T) {
		data := `
host: smtp.example.com
port: 587
username: user
`
		var cfg SMTPConfig
		err := yaml.Unmarshal([]byte(data), &cfg)
		require.NoError(t, err)
		assert.Equal(t, "587", cfg.Port.String())
	})
}
