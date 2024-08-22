// Copyright (C) 2024 The Daguflow/Dagu Authors
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

package dag

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Load(t *testing.T) {
	tests := []struct {
		name             string
		file             string
		expectedError    string
		expectedLocation string
	}{
		{
			name:             "WithExt",
			file:             filepath.Join(testdataDir, "loader_test.yaml"),
			expectedLocation: filepath.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:             "WithoutExt",
			file:             filepath.Join(testdataDir, "loader_test"),
			expectedLocation: filepath.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:          "InvalidPath",
			file:          filepath.Join(testdataDir, "not_existing_file.yaml"),
			expectedError: "no such file or directory",
		},
		{
			name:          "InvalidDAG",
			file:          filepath.Join(testdataDir, "err_decode.yaml"),
			expectedError: "has invalid keys: invalidkey",
		},
		{
			name:          "InvalidYAML",
			file:          filepath.Join(testdataDir, "err_parse.yaml"),
			expectedError: "cannot unmarshal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := Load("", tt.file, "")
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLocation, dg.Location)
			}
		})
	}
}

func Test_LoadMetadata(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		dg, err := LoadMetadata(filepath.Join(testdataDir, "default.yaml"))
		require.NoError(t, err)

		require.Equal(t, dg.Name, "default")
		// Check if steps are empty since we are loading metadata only
		require.True(t, len(dg.Steps) == 0)
	})
}

func Test_loadBaseConfig(t *testing.T) {
	t.Run("LoadBaseConfigFile", func(t *testing.T) {
		base, err := os.ReadFile(filepath.Join(testdataDir, "base.yaml"))
		require.NoError(t, err)

		dg, err := loadBaseConfig(base, buildOpts{})
		require.NotNil(t, dg)
		require.NoError(t, err)
	})
}

func Test_LoadDefaultConfig(t *testing.T) {
	t.Run("DefaultConfigWithoutBaseConfig", func(t *testing.T) {
		file := filepath.Join(testdataDir, "default.yaml")
		dg, err := Load("", file, "")

		require.NoError(t, err)

		// Check if the default values are set correctly
		assert.Equal(t, "", dg.LogDir)
		assert.Equal(t, file, dg.Location)
		assert.Equal(t, "default", dg.Name)
		assert.Equal(t, time.Second*60, dg.MaxCleanUpTime)
		assert.Equal(t, 30, dg.HistRetentionDays)

		// Check if the steps are loaded correctly
		require.Len(t, dg.Steps, 1)
		assert.Equal(t, "1", dg.Steps[0].Name, "1")
		assert.Equal(t, "true", dg.Steps[0].Command, "true")
		assert.Equal(t, filepath.Dir(file), dg.Steps[0].Dir)
	})
}

const (
	testDAG = `
name: test DAG
steps:
  - name: "1"
    command: "true"
`
)

func Test_LoadYAML(t *testing.T) {
	t.Run("ValidYAMLData", func(t *testing.T) {
		ret, err := LoadYAML("test", nil, []byte(testDAG))
		require.NoError(t, err)
		require.Equal(t, ret.Name, "test DAG")

		step := ret.Steps[0]
		require.Equal(t, step.Name, "1")
		require.Equal(t, step.Command, "true")
	})
	t.Run("InvalidYAMLData", func(t *testing.T) {
		_, err := LoadYAML("test", nil, []byte(`invalidyaml`))
		require.Error(t, err)
	})
}
