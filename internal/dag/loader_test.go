package dag

import (
	"path"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
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
			name:             "Load file with .yaml",
			file:             path.Join(testdataDir, "loader_test.yaml"),
			expectedLocation: path.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:             "Load file without .yaml",
			file:             path.Join(testdataDir, "loader_test"),
			expectedLocation: path.Join(testdataDir, "loader_test.yaml"),
		},
		{
			name:          "[Invalid] DAG file does not exist",
			file:          path.Join(testdataDir, "not_existing_file.yaml"),
			expectedError: "no such file or directory",
		},
		{
			name:          "[Invalid] DAG file has invalid keys",
			file:          path.Join(testdataDir, "err_decode.yaml"),
			expectedError: "has invalid keys: invalidkey",
		},
		{
			name:          "[Invalid] DAG file cannot unmarshal",
			file:          path.Join(testdataDir, "err_parse.yaml"),
			expectedError: "cannot unmarshal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Load("", tt.file, "")
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLocation, d.Location)
			}
		})
	}
}

func Test_LoadMetadata(t *testing.T) {
	t.Run("Load metadata", func(t *testing.T) {
		d, err := LoadMetadata(path.Join(testdataDir, "default.yaml"))
		require.NoError(t, err)

		require.Equal(t, d.Name, "default")
		// Check if steps are empty since we are loading metadata only
		require.True(t, len(d.Steps) == 0)
	})
}

func Test_loadBaseConfig(t *testing.T) {
	t.Run("Load base config file", func(t *testing.T) {
		// The base config file is set on the global config
		// This should be `testdata/home/.dagu/config.yaml`.
		d, err := loadBaseConfig(config.Get().BaseConfig, buildOpts{})
		require.NotNil(t, d)
		require.NoError(t, err)
	})
}

func Test_LoadDefaultConfig(t *testing.T) {
	t.Run("Load default config", func(t *testing.T) {
		d, err := Load("", path.Join(testdataDir, "default.yaml"), "")
		require.NoError(t, err)

		// Check if the default values are set correctly
		require.Equal(t, time.Second*60, d.MaxCleanUpTime)
		require.Equal(t, 30, d.HistRetentionDays)
	})
}

func Test_LoadYAML(t *testing.T) {
	t.Run("Load YAML data", func(t *testing.T) {
		dat := `
name: test DAG
steps:
  - name: "1"
    command: "true"
`
		ret, err := LoadYAML([]byte(dat))
		require.NoError(t, err)
		require.Equal(t, ret.Name, "test DAG")

		step := ret.Steps[0]
		require.Equal(t, step.Name, "1")
		require.Equal(t, step.Command, "true")
	})
	t.Run("[Invalid] Load invalid YAML data", func(t *testing.T) {
		dat := `invalidyaml`
		_, err := LoadYAML([]byte(dat))
		require.Error(t, err)
	})
}
