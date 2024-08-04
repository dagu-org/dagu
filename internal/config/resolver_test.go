package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daguflow/dagu/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	t.Run("App home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		os.Setenv("TEST_APP_HOME", filepath.Join(tmpDir, "dagu"))
		r := newResolver("TEST_APP_HOME", filepath.Join(tmpDir, ".dagu"), XDGConfig{})

		assert.Equal(t, r, resolver{
			configDir:       filepath.Join(tmpDir, "dagu"),
			dagsDir:         filepath.Join(tmpDir, "dagu", "dags"),
			suspendFlagsDir: filepath.Join(tmpDir, "dagu", "suspend"),
			dataDir:         filepath.Join(tmpDir, "dagu", "data"),
			logsDir:         filepath.Join(tmpDir, "dagu", "logs"),
			adminLogsDir:    filepath.Join(tmpDir, "dagu", "logs/admin"),
			baseConfigFile:  filepath.Join(tmpDir, "dagu", "base.yaml"),
		})
	})
	t.Run("Legacy home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		legacyPath := filepath.Join(tmpDir, ".dagu")
		err := os.MkdirAll(legacyPath, os.ModePerm)
		require.NoError(t, err)

		r := newResolver("UNSET_APP_HOME", legacyPath, XDGConfig{})

		assert.Equal(t, r, resolver{
			configDir:       filepath.Join(tmpDir, ".dagu"),
			dagsDir:         filepath.Join(tmpDir, ".dagu", "dags"),
			suspendFlagsDir: filepath.Join(tmpDir, ".dagu", "suspend"),
			dataDir:         filepath.Join(tmpDir, ".dagu", "data"),
			logsDir:         filepath.Join(tmpDir, ".dagu", "logs"),
			adminLogsDir:    filepath.Join(tmpDir, ".dagu", "logs", "admin"),
			baseConfigFile:  filepath.Join(tmpDir, ".dagu", "base.yaml"),
		})
	})
	t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
		r := newResolver("UNSET_APP_HOME", ".test", XDGConfig{
			DataHome:   "/home/user/.local/share",
			ConfigHome: "/home/user/.config",
		})
		assert.Equal(t, r, resolver{
			configDir:       "/home/user/.config/dagu",
			dagsDir:         "/home/user/.config/dagu/dags",
			suspendFlagsDir: "/home/user/.local/share/dagu/suspend",
			dataDir:         "/home/user/.local/share/dagu/history",
			logsDir:         "/home/user/.local/share/dagu/logs",
			adminLogsDir:    "/home/user/.local/share/dagu/logs/admin",
			baseConfigFile:  "/home/user/.config/dagu/base.yaml",
		})
	})
}
