package config

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	t.Run("App home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		os.Setenv("TEST_APP_HOME", filepath.Join(tmpDir, build.Slug))
		r := newResolver("TEST_APP_HOME", filepath.Join(tmpDir, ".dagu"), XDGConfig{})

		assert.Equal(t, r, resolver{
			configDir:       filepath.Join(tmpDir, build.Slug),
			dagsDir:         filepath.Join(tmpDir, build.Slug, "dags"),
			suspendFlagsDir: filepath.Join(tmpDir, build.Slug, "suspend"),
			dataDir:         filepath.Join(tmpDir, build.Slug, "data"),
			logsDir:         filepath.Join(tmpDir, build.Slug, "logs"),
			adminLogsDir:    filepath.Join(tmpDir, build.Slug, "logs/admin"),
			baseConfigFile:  filepath.Join(tmpDir, build.Slug, "base.yaml"),
		})
	})
	t.Run("Legacy home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		hiddenDir := filepath.Join(tmpDir, "."+build.Slug)
		legacyPath := filepath.Join(tmpDir, hiddenDir)
		err := os.MkdirAll(legacyPath, os.ModePerm)
		require.NoError(t, err)

		r := newResolver("UNSET_APP_HOME", legacyPath, XDGConfig{})

		assert.Equal(t, r, resolver{
			configDir:       filepath.Join(tmpDir, hiddenDir),
			dagsDir:         filepath.Join(tmpDir, hiddenDir, "dags"),
			suspendFlagsDir: filepath.Join(tmpDir, hiddenDir, "suspend"),
			dataDir:         filepath.Join(tmpDir, hiddenDir, "data"),
			logsDir:         filepath.Join(tmpDir, hiddenDir, "logs"),
			adminLogsDir:    filepath.Join(tmpDir, hiddenDir, "logs", "admin"),
			baseConfigFile:  filepath.Join(tmpDir, hiddenDir, "base.yaml"),
		})
	})
	t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
		r := newResolver("UNSET_APP_HOME", ".test", XDGConfig{
			DataHome:   "/home/user/.local/share",
			ConfigHome: "/home/user/.config",
		})
		assert.Equal(t, r, resolver{
			configDir:       path.Join("/home/user/.config", build.Slug),
			dagsDir:         path.Join("/home/user/.config", build.Slug, "dags"),
			suspendFlagsDir: path.Join("/home/user/.local/share", build.Slug, "suspend"),
			dataDir:         path.Join("/home/user/.local/share", build.Slug, "history"),
			logsDir:         path.Join("/home/user/.local/share", build.Slug, "logs"),
			adminLogsDir:    path.Join("/home/user/.local/share", build.Slug, "logs", "admin"),
			baseConfigFile:  path.Join("/home/user/.config", build.Slug, "base.yaml"),
		})
	})
}
