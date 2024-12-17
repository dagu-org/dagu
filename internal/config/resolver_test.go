// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	t.Parallel()
	t.Run("App home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		os.Setenv("TEST_APP_HOME", filepath.Join(tmpDir, build.Slug))
		r := newResolver("TEST_APP_HOME", filepath.Join(tmpDir, ".dagu"), XDGConfig{})

		assert.Equal(t, r, PathResolver{
			Paths: Paths{
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
	t.Run("Legacy home directory", func(t *testing.T) {
		tmpDir := util.MustTempDir("test")
		defer os.RemoveAll(tmpDir)

		hiddenDir := filepath.Join(tmpDir, "."+build.Slug)
		legacyPath := filepath.Join(tmpDir, hiddenDir)
		err := os.MkdirAll(legacyPath, os.ModePerm)
		require.NoError(t, err)

		r := newResolver("UNSET_APP_HOME", legacyPath, XDGConfig{})

		assert.Equal(t, r, PathResolver{
			Paths: Paths{
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
	t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
		r := newResolver("UNSET_APP_HOME", ".test", XDGConfig{
			DataHome:   "/home/user/.local/share",
			ConfigHome: "/home/user/.config",
		})
		assert.Equal(t, r, PathResolver{
			Paths: Paths{
				ConfigDir:       path.Join("/home/user/.config", build.Slug),
				DAGsDir:         path.Join("/home/user/.config", build.Slug, "dags"),
				SuspendFlagsDir: path.Join("/home/user/.local/share", build.Slug, "suspend"),
				DataDir:         path.Join("/home/user/.local/share", build.Slug, "history"),
				LogsDir:         path.Join("/home/user/.local/share", build.Slug, "logs"),
				AdminLogsDir:    path.Join("/home/user/.local/share", build.Slug, "logs", "admin"),
				BaseConfigFile:  path.Join("/home/user/.config", build.Slug, "base.yaml"),
			},
			XDGConfig: XDGConfig{
				DataHome:   "/home/user/.local/share",
				ConfigHome: "/home/user/.config",
			},
		})
	})
}
