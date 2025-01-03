package digraph

import (
	"context"
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
			dag, err := Load(context.Background(), tt.file)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLocation, dag.Location)
			}
		})
	}
}

func Test_LoadMetadata(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		filePath := filepath.Join(testdataDir, "default.yaml")
		dag, err := Load(context.Background(), filePath, OnlyMetadata(), WithoutEval())
		require.NoError(t, err)

		require.Equal(t, dag.Name, "default")
		// Check if steps are empty since we are loading metadata only
		require.True(t, len(dag.Steps) == 0)
	})
}

func Test_loadBaseConfig(t *testing.T) {
	t.Run("LoadBaseConfigFile", func(t *testing.T) {
		dag, err := loadBaseConfig(BuildContext{}, filepath.Join(testdataDir, "base.yaml"))
		require.NotNil(t, dag)
		require.NoError(t, err)
	})
}

func Test_LoadDefaultConfig(t *testing.T) {
	t.Run("DefaultConfigWithoutBaseConfig", func(t *testing.T) {
		filePath := filepath.Join(testdataDir, "default.yaml")
		dag, err := Load(context.Background(), filePath)

		require.NoError(t, err)

		// Check if the default values are set correctly
		assert.Equal(t, "", dag.LogDir)
		assert.Equal(t, filePath, dag.Location)
		assert.Equal(t, "default", dag.Name)
		assert.Equal(t, time.Second*60, dag.MaxCleanUpTime)
		assert.Equal(t, 30, dag.HistRetentionDays)

		// Check if the steps are loaded correctly
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "1", dag.Steps[0].Name, "1")
		assert.Equal(t, "true", dag.Steps[0].Command, "true")
		assert.Equal(t, filepath.Dir(filePath), dag.Steps[0].Dir)
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
		ret, err := loadYAML(context.Background(), []byte(testDAG), buildOpts{})
		require.NoError(t, err)
		require.Equal(t, ret.Name, "test DAG")

		step := ret.Steps[0]
		require.Equal(t, step.Name, "1")
		require.Equal(t, step.Command, "true")
	})
	t.Run("InvalidYAMLData", func(t *testing.T) {
		_, err := loadYAML(context.Background(), []byte(`invalidyaml`), buildOpts{})
		require.Error(t, err)
	})
}
