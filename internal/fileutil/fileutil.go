package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

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

// FileExists returns true if file exists.
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// OpenOrCreateFile opens file or creates it if it doesn't exist.
func OpenOrCreateFile(file string) (*os.File, error) {
	if FileExists(file) {
		return openFile(file)
	}
	return createFile(file)
}

// openFile opens file.
func openFile(file string) (*os.File, error) {
	outfile, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return outfile, nil
}

// createFile creates file.
func createFile(file string) (*os.File, error) {
	outfile, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	return outfile, nil
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
