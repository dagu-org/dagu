package main_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenLogFile(t *testing.T) {
	t.Run("successful log file creation", func(t *testing.T) {
		tempDir := t.TempDir() // Using t.TempDir() for automatic cleanup

		ctx := context.Background()
		setup := cmd.SetupWithConfig(ctx, &config.Config{
			Paths: config.PathsConfig{LogDir: tempDir},
		})

		file, err := setup.OpenLogFile(ctx, "test_", &digraph.DAG{
			Name:   "test_dag",
			LogDir: "",
		}, "12345678")
		require.NoError(t, err)
		defer file.Close()

		assert.NotNil(t, file)
		assert.True(t, filepath.IsAbs(file.Name()))
		assert.Contains(t, file.Name(), "test_dag")
		assert.Contains(t, file.Name(), "test_")
		assert.Contains(t, file.Name(), "12345678")
	})
}

func TestSetupLogDirectory(t *testing.T) {
	tests := []struct {
		name     string
		config   cmd.LogFileSettings
		wantErr  bool
		validate func(t *testing.T, path string)
	}{
		{
			name: "using LogDir",
			config: cmd.LogFileSettings{
				LogDir:  t.TempDir(),
				DAGName: "test_dag",
			},
			validate: func(t *testing.T, path string) {
				assert.Contains(t, path, "test_dag")
				assert.DirExists(t, path)
			},
		},
		{
			name: "using DAGLogDir",
			config: cmd.LogFileSettings{
				DAGLogDir: filepath.Join(t.TempDir(), "custom"),
				DAGName:   "test_dag",
			},
			validate: func(t *testing.T, path string) {
				assert.Contains(t, path, "custom")
				assert.Contains(t, path, "test_dag")
				assert.DirExists(t, path)
			},
		},
		{
			name: "with special characters in DAGName",
			config: cmd.LogFileSettings{
				LogDir:  t.TempDir(),
				DAGName: "test/dag*special",
			},
			validate: func(t *testing.T, path string) {
				assert.NotContains(t, path, "/dag*")
				assert.DirExists(t, path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cmd.SetupLogDirectory(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestBuildLogFilename(t *testing.T) {
	t.Run("filename format", func(t *testing.T) {
		config := cmd.LogFileSettings{
			Prefix:    "test_",
			DAGName:   "test dag",
			RequestID: "12345678901234", // Longer than 8 chars to test truncation
		}

		filename := cmd.BuildLogFilename(config)

		assert.Contains(t, filename, "test_")
		assert.Contains(t, filename, "test_dag")
		assert.Contains(t, filename, time.Now().Format("20060102"))
		assert.Contains(t, filename, "12345678")  // Should be truncated
		assert.NotContains(t, filename, "901234") // Shouldn't contain the rest
		assert.Contains(t, filename, ".log")

		// Test proper sanitization
		assert.NotContains(t, filename, " ") // Space should be replaced
	})
}

func TestCreateLogFile(t *testing.T) {
	t.Run("file creation and permissions", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.log")

		file, err := cmd.CreateLogFile(filePath)
		require.NoError(t, err)
		defer file.Close()

		assert.NotNil(t, file)
		assert.Equal(t, filePath, file.Name())

		info, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})

	t.Run("invalid path", func(t *testing.T) {
		_, err := cmd.CreateLogFile("/nonexistent/directory/test.log")
		assert.Error(t, err)
	})
}

func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name    string
		config  cmd.LogFileSettings
		wantErr bool
	}{
		{
			name: "valid settings",
			config: cmd.LogFileSettings{
				LogDir:  "/tmp",
				DAGName: "test",
			},
			wantErr: false,
		},
		{
			name: "empty DAGName",
			config: cmd.LogFileSettings{
				LogDir: "/tmp",
			},
			wantErr: true,
		},
		{
			name: "no directories",
			config: cmd.LogFileSettings{
				DAGName: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.ValidateSettings(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
