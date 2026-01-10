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

func TestIsFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("RegularFile", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(tmpDir, "testfile.txt")
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)

		require.True(t, IsFile(filePath))
	})

	t.Run("Directory", func(t *testing.T) {
		t.Parallel()

		dirPath := filepath.Join(tmpDir, "testdir")
		err := os.Mkdir(dirPath, 0755)
		require.NoError(t, err)

		require.False(t, IsFile(dirPath))
	})

	t.Run("NonExistent", func(t *testing.T) {
		t.Parallel()

		require.False(t, IsFile(filepath.Join(tmpDir, "nonexistent")))
	})
}

func TestCreateTempDAGFile(t *testing.T) {
	t.Run("BasicFile", func(t *testing.T) {
		yamlData := []byte("name: test-dag\nsteps:\n  - name: step1\n    command: echo test")
		path, err := CreateTempDAGFile("test-subdir", "test-dag", yamlData)
		require.NoError(t, err)
		require.NotEmpty(t, path)
		t.Cleanup(func() { _ = os.Remove(path) })

		// Verify file exists
		assert.FileExists(t, path)

		// Verify content
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, yamlData, content)

		// Verify path contains expected components
		assert.Contains(t, path, "test-dag")
		assert.Contains(t, path, ".yaml")
		assert.Contains(t, path, filepath.Join("dagu", "test-subdir"))
	})

	t.Run("WithExtraDocs", func(t *testing.T) {
		primaryDoc := []byte("name: parent-dag\nsteps:\n  - name: step1")
		extraDoc1 := []byte("name: child1\nsteps:\n  - name: s1")
		extraDoc2 := []byte("name: child2\nsteps:\n  - name: s2")

		path, err := CreateTempDAGFile("test-subdir", "parent-dag", primaryDoc, extraDoc1, extraDoc2)
		require.NoError(t, err)
		require.NotEmpty(t, path)
		t.Cleanup(func() { _ = os.Remove(path) })

		// Verify file exists
		assert.FileExists(t, path)

		// Verify content has all docs separated by ---
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "name: parent-dag")
		assert.Contains(t, contentStr, "---")
		assert.Contains(t, contentStr, "name: child1")
		assert.Contains(t, contentStr, "name: child2")
	})

	t.Run("EmptyExtraDocs", func(t *testing.T) {
		primaryDoc := []byte("name: solo-dag")

		path, err := CreateTempDAGFile("test-subdir", "solo-dag", primaryDoc)
		require.NoError(t, err)
		require.NotEmpty(t, path)
		t.Cleanup(func() { _ = os.Remove(path) })

		// Verify content has only the primary doc (no separators)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, primaryDoc, content)
		assert.NotContains(t, string(content), "---")
	})
}
