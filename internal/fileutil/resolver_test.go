package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileResolver(t *testing.T) {
	// Create temporary test directories
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")

	// Create test directories
	for _, dir := range []string{dir1, dir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(dir1, "file1.txt"):            "content1",
		filepath.Join(dir2, "file2.txt"):            "content2",
		filepath.Join(dir1, "shared.txt"):           "content_dir1",
		filepath.Join(dir2, "shared.txt"):           "content_dir2",
		filepath.Join(tempDir, "absolute.txt"):      "absolute",
		filepath.Join(os.TempDir(), "homefile.txt"): "home",
	}

	for path, content := range testFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tests := []struct {
		name          string
		relativeTos   []string
		file          string
		expectedPath  string
		expectedError bool
	}{
		{
			name:         "Find file in first relative path",
			relativeTos:  []string{dir1, dir2},
			file:         "file1.txt",
			expectedPath: filepath.Join(dir1, "file1.txt"),
		},
		{
			name:         "Find file in second relative path",
			relativeTos:  []string{dir1, dir2},
			file:         "file2.txt",
			expectedPath: filepath.Join(dir2, "file2.txt"),
		},
		{
			name:         "Find shared file (should use first match)",
			relativeTos:  []string{dir1, dir2},
			file:         "shared.txt",
			expectedPath: filepath.Join(dir1, "shared.txt"),
		},
		{
			name:         "Absolute path exists",
			relativeTos:  []string{dir1, dir2},
			file:         filepath.Join(tempDir, "absolute.txt"),
			expectedPath: filepath.Join(tempDir, "absolute.txt"),
		},
		{
			name:          "Absolute path does not exist",
			relativeTos:   []string{dir1, dir2},
			file:          filepath.Join(tempDir, "nonexistent.txt"),
			expectedError: true,
		},
		{
			name:          "File not found in any location",
			relativeTos:   []string{dir1, dir2},
			file:          "nonexistent.txt",
			expectedError: true,
		},
		{
			name:          "Empty relative paths",
			relativeTos:   []string{},
			file:          "file1.txt",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewFileResolver(tt.relativeTos)
			path, err := resolver.ResolveFilePath(tt.file)

			// Check error cases
			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				return
			}

			// Check success cases
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if path != tt.expectedPath {
				t.Errorf("expected path %s but got %s", tt.expectedPath, path)
			}

			// Verify the file exists
			if !FileExists(path) {
				t.Errorf("resolved path %s does not exist", path)
			}
		})
	}
}

func TestFileNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		err         *FileNotFoundError
		expectedMsg string
	}{
		{
			name: "Error with no searched paths",
			err: &FileNotFoundError{
				Path: "test.txt",
			},
			expectedMsg: "file not found: test.txt",
		},
		{
			name: "Error with searched paths",
			err: &FileNotFoundError{
				Path:          "test.txt",
				SearchedPaths: []string{"/path1", "/path2"},
			},
			expectedMsg: "file not found: test.txt (searched in: [/path1 /path2])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if msg != tt.expectedMsg {
				t.Errorf("expected message %q but got %q", tt.expectedMsg, msg)
			}
		})
	}
}
