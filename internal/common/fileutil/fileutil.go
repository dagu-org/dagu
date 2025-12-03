package fileutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Common errors for file operations
var (
	ErrUnexpectedEOF         = errors.New("unexpected end of input after escape character")
	ErrUnknownEscapeSequence = errors.New("unknown escape sequence")
)

// MustGetUserHomeDir returns current working directory.
// Panics is os.UserHomeDir() returns error
func MustGetUserHomeDir() string {
	hd, _ := os.UserHomeDir()
	return hd
}

// MustGetwd returns current working directory.
// Panics is os.Getwd() returns error
func MustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

// IsDir returns true if path is a directory.
func IsDir(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// FileExists reports whether the named file exists.
// It returns false if os.Stat reports the file does not exist and true otherwise (including when os.Stat returns a different error).
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// IsFile reports whether the named path exists and is a regular file.
// It returns false if the path does not exist or if an error occurs while obtaining file info.
func IsFile(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular()
}

// OpenOrCreateFile opens (or creates) the log file with flags for creation, write-only access,
// OpenOrCreateFile opens or creates the named file for appending with synchronous I/O and sets permissions to 0600.
// It returns the opened *os.File or a non-nil error if the operation fails.
func OpenOrCreateFile(filepath string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_SYNC
	file, err := os.OpenFile(filepath, flags, 0600) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file %s: %w", filepath, err)
	}

	return file, nil
}

// MustTempDir returns temporary directory.
// This function is used only for testing.
func MustTempDir(pattern string) string {
	t, err := os.MkdirTemp("", pattern)
	if err != nil {
		panic(err)
	}
	return t
}

// TruncString TurnString returns truncated string.
func TruncString(val string, max int) string {
	if len(val) > max {
		return val[:max]
	}
	return val
}

const (
	yamlExtension = ".yaml"
	ymlExtension  = ".yml"
)

// ValidYAMLExtensions contains valid YAML extensions.
var ValidYAMLExtensions = []string{yamlExtension, ymlExtension}

// IsYAMLFile checks if a file has a valid YAML extension (.yaml or .yml).
// Returns false for empty strings or files without extensions.
func IsYAMLFile(filename string) bool {
	if filename == "" {
		return false
	}
	return slices.Contains(ValidYAMLExtensions, filepath.Ext(filename))
}

// TrimYAMLFileExtension trims the .yml or .yaml extension from a filename.
func TrimYAMLFileExtension(filename string) string {
	if filename == "" {
		return ""
	}

	ext := filepath.Ext(filename)
	switch ext {
	case ymlExtension:
		return strings.TrimSuffix(filename, ymlExtension)
	case yamlExtension:
		return strings.TrimSuffix(filename, yamlExtension)
	default:
		return filename
	}
}

// IsFileWithExtension is a more generic function that checks if a file
// has any of the provided extensions.
func IsFileWithExtension(filename string, validExtensions []string) bool {
	if filename == "" || len(validExtensions) == 0 {
		return false
	}
	return slices.Contains(validExtensions, filepath.Ext(filename))
}

// EnsureYAMLExtension adds .yaml extension if not present
// if it has .yml extension, replace it with .yaml
func EnsureYAMLExtension(filename string) string {
	if filename == "" {
		return ""
	}

	ext := filepath.Ext(filename)
	switch ext {
	case ymlExtension, yamlExtension:
		return filename

	default:
		return filename + yamlExtension
	}
}

// ResolvePath resolves a path to an absolute path.
// It handles empty paths, tilde expansion, environment variables,
// and converts to an absolute path.
func ResolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)

	// Handle empty path case
	if path == "" {
		return "", nil
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Expand tilde to user's home directory
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Clean the path
	cleanPath := filepath.Clean(absPath)

	return cleanPath, nil
}

// ResolvePathOrBlank works like ResolvePath but panics on error.
// Useful when you're confident the path resolution will succeed.
func ResolvePathOrBlank(path string) string {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		log.Println("Failed to resolve path:", err)
		return path
	}
	return resolvedPath
}