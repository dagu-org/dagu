package fileutil

import (
	"path"
	"strings"
)

// NormalizeDAGPath normalizes a DAG path by cleaning it and removing redundant elements
func NormalizeDAGPath(dagPath string) string {
	// Clean the path
	dagPath = path.Clean(dagPath)

	// Remove leading slash
	dagPath = strings.TrimPrefix(dagPath, "/")

	// Remove trailing slash
	dagPath = strings.TrimSuffix(dagPath, "/")

	// Handle empty or dot path
	if dagPath == "" || dagPath == "." {
		return ""
	}

	return dagPath
}

// SplitDAGPath splits a DAG path into prefix and name components
// Examples:
//   - "workflow/task1" -> ("workflow", "task1")
//   - "data/extract/users" -> ("data/extract", "users")
//   - "task1" -> ("", "task1")
//   - "workflow/" -> ("workflow", "")
func SplitDAGPath(dagPath string) (prefix, name string) {
	// Handle special case of trailing slash before normalization
	hasTrailingSlash := strings.HasSuffix(dagPath, "/") && dagPath != "/"

	dagPath = NormalizeDAGPath(dagPath)
	if dagPath == "" {
		return "", ""
	}

	lastSlash := strings.LastIndex(dagPath, "/")
	if lastSlash == -1 {
		// No prefix, just name
		if hasTrailingSlash {
			// Original had trailing slash, so this is a prefix with empty name
			return dagPath, ""
		}
		return "", dagPath
	}

	return dagPath[:lastSlash], dagPath[lastSlash+1:]
}

// JoinDAGPath joins a prefix and name into a full DAG path
func JoinDAGPath(prefix, name string) string {
	prefix = NormalizeDAGPath(prefix)
	name = strings.TrimSpace(name)

	if prefix == "" {
		return name
	}
	if name == "" {
		return prefix
	}

	return prefix + "/" + name
}

// GetParentPrefix returns the parent prefix of a given prefix
// Examples:
//   - "workflow/extract" -> "workflow"
//   - "workflow" -> ""
//   - "" -> ""
func GetParentPrefix(prefix string) string {
	prefix = NormalizeDAGPath(prefix)
	if prefix == "" {
		return ""
	}

	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash == -1 {
		return ""
	}

	return prefix[:lastSlash]
}

// IsValidDAGPath checks if a DAG path is valid
func IsValidDAGPath(dagPath string) bool {
	if dagPath == "" {
		return false
	}

	// Allow spaces in the overall path, but check components separately
	dagPath = strings.TrimSpace(dagPath)

	// Check for invalid characters
	invalidChars := []string{"..", "\\", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(dagPath, char) {
			return false
		}
	}

	// Check each path component
	parts := strings.Split(dagPath, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
		// Check for leading/trailing spaces in components
		if part != strings.TrimSpace(part) {
			return false
		}
	}

	return true
}
