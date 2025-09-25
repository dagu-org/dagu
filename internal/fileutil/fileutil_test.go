package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFile(t *testing.T) {
	t.Run("FileCreationAndPermissions", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.log")

		file, err := OpenOrCreateFile(filePath)
		require.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()

		assert.NotNil(t, file)
		assert.Equal(t, filePath, file.Name())

		info, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("InvalidPath", func(t *testing.T) {
		_, err := OpenOrCreateFile("/nonexistent/directory/test.log")
		assert.Error(t, err)
	})
}

func TestResolvePath(t *testing.T) {
	// Save original environment to restore later
	origHome := os.Getenv("HOME")
	origTempDir := os.Getenv("TEMP_DIR")
	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("TEMP_DIR", origTempDir)
	}()

	// Set up test environment variables
	testHome := "/test/home"
	testTempDir := "/test/temp"
	_ = os.Setenv("HOME", testHome)
	_ = os.Setenv("TEMP_DIR", testTempDir)

	// Get current working directory for absolute path tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expected    string
		expectError bool
	}{
		{
			name:        "EmptyPath",
			path:        "",
			expected:    "",
			expectError: false,
		},
		{
			name:        "TildeExpansion",
			path:        "~/documents",
			expected:    filepath.Clean(filepath.Join(testHome, "documents")),
			expectError: false,
		},
		{
			name:        "TildeOnly",
			path:        "~",
			expected:    filepath.Clean(testHome),
			expectError: false,
		},
		{
			name:        "EnvironmentVariableExpansion",
			path:        "$TEMP_DIR/logs",
			expected:    filepath.Clean(filepath.Join(testTempDir, "logs")),
			expectError: false,
		},
		{
			name:        "MultipleEnvironmentVariables",
			path:        "$HOME/projects/$TEMP_DIR",
			expected:    filepath.Clean(filepath.Join(testHome, "projects", testTempDir)),
			expectError: false,
		},
		{
			name:        "PathCleaningWithDots",
			path:        "/usr/local/../bin/./app",
			expected:    "/usr/bin/app",
			expectError: false,
		},
		{
			name:        "PathCleaningWithRedundantSlashes",
			path:        "/usr//local/bin",
			expected:    "/usr/local/bin",
			expectError: false,
		},
		{
			name:        "CombinedTildeAndEnvironmentVariable",
			path:        "~/projects/$TEMP_DIR",
			expected:    filepath.Clean(filepath.Join(testHome, "projects", testTempDir)),
			expectError: false,
		},
		{
			name:        "AbsolutePath",
			path:        "/usr/local/bin",
			expected:    "/usr/local/bin",
			expectError: false,
		},
		{
			name:        "RelativePath",
			path:        "projects/dagu",
			expected:    filepath.Join(cwd, "projects/dagu"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePath(tt.path)

			// Check error expectation
			if (err != nil) != tt.expectError {
				t.Errorf("ResolvePath(%q) error = %v, expectError %v", tt.path, err, tt.expectError)
				return
			}

			if tt.expectError {
				return // No need to check the result if we expected an error
			}

			// For empty path, we expect empty result
			if tt.path == "" {
				if result != "" {
					t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, result, "")
				}
				return
			}

			// For all other paths, check the result matches expected
			if result != tt.expected {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestMustResolvePath(t *testing.T) {
	// Test normal case
	t.Run("NormalCase", func(t *testing.T) {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current working directory: %v", err)
		}

		path := "test.txt"
		expected := filepath.Join(cwd, path)

		result := ResolvePathOrBlank(path)
		if result != expected {
			t.Errorf("MustResolvePath(%q) = %q, want %q", path, result, expected)
		}
	})

	// Test panic case - can't easily test without mocking os functions
	// but we can at least verify it calls ResolvePath
	t.Run("CallsResolvePath", func(t *testing.T) {
		path := "test.txt"
		resolved, err := ResolvePath(path)
		if err != nil {
			t.Fatalf("ResolvePath failed: %v", err)
		}

		mustResolved := ResolvePathOrBlank(path)
		if mustResolved != resolved {
			t.Errorf("MustResolvePath(%q) = %q, but ResolvePath(%q) = %q",
				path, mustResolved, path, resolved)
		}
	})
}
