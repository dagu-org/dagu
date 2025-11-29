package config_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {
	t.Parallel()
	t.Run("AppHomeDirectory", func(t *testing.T) {
		t.Parallel()
		tmpDir := fileutil.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		_ = os.Setenv("TEST_APP_HOME", filepath.Join(tmpDir, config.AppSlug))
		paths, err := config.ResolvePaths("TEST_APP_HOME", filepath.Join(tmpDir, ".dagu"), config.XDGConfig{})
		require.NoError(t, err)

		assert.Equal(t, config.Paths{
			ConfigDir:       filepath.Join(tmpDir, config.AppSlug),
			DAGsDir:         filepath.Join(tmpDir, config.AppSlug, "dags"),
			SuspendFlagsDir: filepath.Join(tmpDir, config.AppSlug, "suspend"),
			DataDir:         filepath.Join(tmpDir, config.AppSlug, "data"),
			LogsDir:         filepath.Join(tmpDir, config.AppSlug, "logs"),
			AdminLogsDir:    filepath.Join(tmpDir, config.AppSlug, "logs/admin"),
			BaseConfigFile:  filepath.Join(tmpDir, config.AppSlug, "base.yaml"),
		}, paths)
	})
	t.Run("AppHomeDirectoryRelativePath", func(t *testing.T) {
		t.Parallel()
		tmpDir := fileutil.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		// Change to tmpDir so relative path resolves correctly
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		err = os.Chdir(tmpDir)
		require.NoError(t, err)
		defer os.Chdir(originalWd)

		// Set a relative path
		relativePath := config.AppSlug
		_ = os.Setenv("TEST_APP_HOME_REL", relativePath)
		paths, err := config.ResolvePaths("TEST_APP_HOME_REL", filepath.Join(tmpDir, ".dagu"), config.XDGConfig{})
		require.NoError(t, err)

		// Should be converted to absolute path
		expectedAbsPath := filepath.Join(tmpDir, config.AppSlug)
		assert.Equal(t, expectedAbsPath, paths.ConfigDir)

		// Environment variable should be updated to absolute path
		assert.Equal(t, expectedAbsPath, os.Getenv("TEST_APP_HOME_REL"))
	})
	t.Run("UnifiedHomeDirectory", func(t *testing.T) {
		t.Parallel()
		tmpDir := fileutil.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		hiddenDir := filepath.Join(tmpDir, "."+config.AppSlug)
		legacyPath := filepath.Join(tmpDir, hiddenDir)
		err := os.MkdirAll(legacyPath, os.ModePerm)
		require.NoError(t, err)

		paths, err := config.ResolvePaths("UNSET_APP_HOME", legacyPath, config.XDGConfig{})
		require.NoError(t, err)

		assert.Equal(t, config.Paths{
			ConfigDir:       filepath.Join(tmpDir, hiddenDir),
			DAGsDir:         filepath.Join(tmpDir, hiddenDir, "dags"),
			SuspendFlagsDir: filepath.Join(tmpDir, hiddenDir, "suspend"),
			DataDir:         filepath.Join(tmpDir, hiddenDir, "data"),
			LogsDir:         filepath.Join(tmpDir, hiddenDir, "logs"),
			AdminLogsDir:    filepath.Join(tmpDir, hiddenDir, "logs", "admin"),
			BaseConfigFile:  filepath.Join(tmpDir, hiddenDir, "base.yaml"),
			Warnings:        []string{"Warning: Dagu legacy directory (" + legacyPath + ") structure detected. This is deprecated."},
		}, paths)
	})
	t.Run("XDGCONFIGHOME", func(t *testing.T) {
		t.Parallel()
		paths, err := config.ResolvePaths("UNSET_APP_HOME", ".test", config.XDGConfig{
			DataHome:   "/home/user/.local/share",
			ConfigHome: "/home/user/.config",
		})
		require.NoError(t, err)
		assert.Equal(t, config.Paths{
			ConfigDir:       path.Join("/home/user/.config", config.AppSlug),
			DAGsDir:         path.Join("/home/user/.config", config.AppSlug, "dags"),
			SuspendFlagsDir: path.Join("/home/user/.local/share", config.AppSlug, "suspend"),
			DataDir:         path.Join("/home/user/.local/share", config.AppSlug, "data"),
			LogsDir:         path.Join("/home/user/.local/share", config.AppSlug, "logs"),
			AdminLogsDir:    path.Join("/home/user/.local/share", config.AppSlug, "logs", "admin"),
			BaseConfigFile:  path.Join("/home/user/.config", config.AppSlug, "base.yaml"),
		}, paths)
	})
}
