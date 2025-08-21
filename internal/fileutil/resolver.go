package fileutil

import (
	"fmt"
	"path/filepath"
)

// FileResolver handles file path resolution across multiple locations
type FileResolver struct {
	relativeTos []string
}

// NewFileResolver creates a new FileResolver instance
func NewFileResolver(relativeTos []string) *FileResolver {
	return &FileResolver{
		relativeTos: relativeTos,
	}
}

// ResolveFilePath attempts to find a file in multiple locations in the following order:
func (r *FileResolver) ResolveFilePath(file string) (string, error) {
	// Check if it's an absolute path
	if filepath.IsAbs(file) {
		if FileExists(file) {
			return file, nil
		}
		return "", &FileNotFoundError{Path: file}
	}

	// Get search locations
	searchPaths, err := r.getSearchPaths(file)
	if err != nil {
		return "", fmt.Errorf("getting search paths: %w", err)
	}

	// Try each location
	for _, path := range searchPaths {
		if FileExists(path) {
			return path, nil
		}
	}

	return "", &FileNotFoundError{
		Path:          file,
		SearchedPaths: searchPaths,
	}
}

// getSearchPaths returns a list of paths to search for the file
func (r *FileResolver) getSearchPaths(file string) ([]string, error) {
	var paths []string

	for _, relativeTo := range r.relativeTos {
		if IsDir(relativeTo) {
			paths = append(paths, filepath.Join(relativeTo, file))
		} else {
			dir := filepath.Dir(relativeTo)
			paths = append(paths, filepath.Join(dir, file))
		}
	}

	return paths, nil
}

// FileNotFoundError provides detailed information about file search failure
type FileNotFoundError struct {
	Path          string
	SearchedPaths []string
}

func (e *FileNotFoundError) Error() string {
	if len(e.SearchedPaths) == 0 {
		return fmt.Sprintf("file not found: %s", e.Path)
	}
	return fmt.Sprintf("file not found: %s (searched in: %v)", e.Path, e.SearchedPaths)
}
