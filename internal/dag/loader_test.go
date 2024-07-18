package dag

import (
	"path"
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
			file:             path.Join(testdataDir, "loader_test.yaml"),
			expectedLocation: path.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:             "WithoutExt",
			file:             path.Join(testdataDir, "loader_test"),
			expectedLocation: path.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:          "InvalidPath",
			file:          path.Join(testdataDir, "not_existing_file.yaml"),
			expectedError: "no such file or directory",
		},
		{
			name:          "InvalidDAG",
			file:          path.Join(testdataDir, "err_decode.yaml"),
			expectedError: "has invalid keys: invalidkey",
		},
		{
			name:          "InvalidYAML",
			file:          path.Join(testdataDir, "err_parse.yaml"),
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
		dg, err := LoadMetadata(path.Join(testdataDir, "default.yaml"))
		require.NoError(t, err)

		require.Equal(t, dg.Name, "default")
		// Check if steps are empty since we are loading metadata only
		require.True(t, len(dg.Steps) == 0)
	})
}

func Test_loadBaseConfig(t *testing.T) {
	t.Run("LoadBaseConfigFile", func(t *testing.T) {
		dg, err := loadBaseConfig(filepath.Join(testdataDir, "base.yaml"), buildOpts{})
		require.NotNil(t, dg)
		require.NoError(t, err)
	})
}

func Test_LoadDefaultConfig(t *testing.T) {
	t.Run("DefaultConfigWithoutBaseConfig", func(t *testing.T) {
		file := path.Join(testdataDir, "default.yaml")
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
		assert.Equal(t, path.Dir(file), dg.Steps[0].Dir)
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
		ret, err := LoadYAML([]byte(testDAG))
		require.NoError(t, err)
		require.Equal(t, ret.Name, "test DAG")

		step := ret.Steps[0]
		require.Equal(t, step.Name, "1")
		require.Equal(t, step.Command, "true")
	})
	t.Run("InvalidYAMLData", func(t *testing.T) {
		_, err := LoadYAML([]byte(`invalidyaml`))
		require.Error(t, err)
	})
}
