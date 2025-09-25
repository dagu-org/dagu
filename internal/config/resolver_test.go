package config_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	t.Parallel()
	t.Run("AppHomeDirectory", func(t *testing.T) {
		tmpDir := fileutil.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		_ = os.Setenv("TEST_APP_HOME", filepath.Join(tmpDir, build.Slug))
		r := config.NewResolver("TEST_APP_HOME", filepath.Join(tmpDir, ".dagu"), config.XDGConfig{})

		assert.Equal(t, r, config.PathResolver{
			Paths: config.Paths{
				ConfigDir:       filepath.Join(tmpDir, build.Slug),
				DAGsDir:         filepath.Join(tmpDir, build.Slug, "dags"),
				SuspendFlagsDir: filepath.Join(tmpDir, build.Slug, "suspend"),
				DataDir:         filepath.Join(tmpDir, build.Slug, "data"),
				LogsDir:         filepath.Join(tmpDir, build.Slug, "logs"),
				AdminLogsDir:    filepath.Join(tmpDir, build.Slug, "logs/admin"),
				BaseConfigFile:  filepath.Join(tmpDir, build.Slug, "base.yaml"),
			},
		})
	})
	t.Run("LegacyHomeDirectory", func(t *testing.T) {
		tmpDir := fileutil.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		hiddenDir := filepath.Join(tmpDir, "."+build.Slug)
		legacyPath := filepath.Join(tmpDir, hiddenDir)
		err := os.MkdirAll(legacyPath, os.ModePerm)
		require.NoError(t, err)

		r := config.NewResolver("UNSET_APP_HOME", legacyPath, config.XDGConfig{})

		assert.Equal(t, r, config.PathResolver{
			Paths: config.Paths{
				ConfigDir:       filepath.Join(tmpDir, hiddenDir),
				DAGsDir:         filepath.Join(tmpDir, hiddenDir, "dags"),
				SuspendFlagsDir: filepath.Join(tmpDir, hiddenDir, "suspend"),
				DataDir:         filepath.Join(tmpDir, hiddenDir, "data"),
				LogsDir:         filepath.Join(tmpDir, hiddenDir, "logs"),
				AdminLogsDir:    filepath.Join(tmpDir, hiddenDir, "logs", "admin"),
				BaseConfigFile:  filepath.Join(tmpDir, hiddenDir, "base.yaml"),
			},
		})
	})
	t.Run("XDGCONFIGHOME", func(t *testing.T) {
		r := config.NewResolver("UNSET_APP_HOME", ".test", config.XDGConfig{
			DataHome:   "/home/user/.local/share",
			ConfigHome: "/home/user/.config",
		})
		assert.Equal(t, r, config.PathResolver{
			Paths: config.Paths{
				ConfigDir:       path.Join("/home/user/.config", build.Slug),
				DAGsDir:         path.Join("/home/user/.config", build.Slug, "dags"),
				SuspendFlagsDir: path.Join("/home/user/.local/share", build.Slug, "suspend"),
				DataDir:         path.Join("/home/user/.local/share", build.Slug, "data"),
				LogsDir:         path.Join("/home/user/.local/share", build.Slug, "logs"),
				AdminLogsDir:    path.Join("/home/user/.local/share", build.Slug, "logs", "admin"),
				BaseConfigFile:  path.Join("/home/user/.config", build.Slug, "base.yaml"),
			},
			XDGConfig: config.XDGConfig{
				DataHome:   "/home/user/.local/share",
				ConfigHome: "/home/user/.config",
			},
		})
	})
}
