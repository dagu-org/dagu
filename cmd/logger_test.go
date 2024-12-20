// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenLogFile(t *testing.T) {
	t.Run("successful log file creation", func(t *testing.T) {
		tempDir := t.TempDir() // Using t.TempDir() for automatic cleanup

		config := logFileSettings{
			Prefix:    "test_",
			LogDir:    tempDir,
			DAGName:   "test_dag",
			RequestID: "12345678",
		}

		file, err := openLogFile(config)
		require.NoError(t, err)
		defer file.Close()

		assert.NotNil(t, file)
		assert.True(t, filepath.IsAbs(file.Name()))
		assert.Contains(t, file.Name(), "test_dag")
		assert.Contains(t, file.Name(), "test_")
		assert.Contains(t, file.Name(), "12345678")
	})

	t.Run("invalid settings", func(t *testing.T) {
		invalidConfigs := []struct {
			name   string
			config logFileSettings
		}{
			{
				name:   "empty DAGName",
				config: logFileSettings{LogDir: "dir"},
			},
			{
				name:   "no directories specified",
				config: logFileSettings{DAGName: "dag"},
			},
		}

		for _, tc := range invalidConfigs {
			t.Run(tc.name, func(t *testing.T) {
				_, err := openLogFile(tc.config)
				assert.Error(t, err)
			})
		}
	})
}

func TestSetupLogDirectory(t *testing.T) {
	tests := []struct {
		name     string
		config   logFileSettings
		wantErr  bool
		validate func(t *testing.T, path string)
	}{
		{
			name: "using LogDir",
			config: logFileSettings{
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
			config: logFileSettings{
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
			config: logFileSettings{
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
			result, err := setupLogDirectory(tt.config)
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
		config := logFileSettings{
			Prefix:    "test_",
			DAGName:   "test dag",
			RequestID: "12345678901234", // Longer than 8 chars to test truncation
		}

		filename := buildLogFilename(config)

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

		file, err := createLogFile(filePath)
		require.NoError(t, err)
		defer file.Close()

		assert.NotNil(t, file)
		assert.Equal(t, filePath, file.Name())

		info, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})

	t.Run("invalid path", func(t *testing.T) {
		_, err := createLogFile("/nonexistent/directory/test.log")
		assert.Error(t, err)
	})
}

func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name    string
		config  logFileSettings
		wantErr bool
	}{
		{
			name: "valid settings",
			config: logFileSettings{
				LogDir:  "/tmp",
				DAGName: "test",
			},
			wantErr: false,
		},
		{
			name: "empty DAGName",
			config: logFileSettings{
				LogDir: "/tmp",
			},
			wantErr: true,
		},
		{
			name: "no directories",
			config: logFileSettings{
				DAGName: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSettings(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
