package fileagentsoul

import (
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//go:embed examples/*.md
var exampleSoulsFS embed.FS

const examplesMarkerFile = ".examples-created"

// ExampleSoulIDs returns the IDs of bundled example souls.
func ExampleSoulIDs() []string {
	return []string{"default"}
}

// SeedExampleSouls writes bundled example souls to baseDir if not already seeded.
// Returns true if examples were created this call.
func SeedExampleSouls(baseDir string) bool {
	markerPath := filepath.Join(baseDir, examplesMarkerFile)
	if _, err := os.Stat(markerPath); err == nil {
		return false // already seeded
	}
	if hasExistingSouls(baseDir) {
		return false
	}

	if err := os.MkdirAll(baseDir, soulDirPermissions); err != nil {
		slog.Warn("Failed to create souls directory", "dir", baseDir, "error", err)
		return false
	}

	slog.Info("Creating example souls for first-time users", "dir", baseDir)

	entries, err := exampleSoulsFS.ReadDir("examples")
	if err != nil {
		slog.Warn("Failed to read embedded example souls", "error", err)
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, readErr := exampleSoulsFS.ReadFile("examples/" + entry.Name())
		if readErr != nil {
			slog.Warn("Failed to read embedded example soul", "name", entry.Name(), "error", readErr)
			continue
		}

		destPath := filepath.Join(baseDir, entry.Name())
		if _, statErr := os.Stat(destPath); statErr == nil {
			continue // don't overwrite existing files
		}

		if err := os.WriteFile(destPath, data, filePermissions); err != nil {
			slog.Warn("Failed to write example soul", "path", destPath, "error", err)
		}
	}

	markerContent := []byte("# This file indicates that example souls have been created.\n# Delete this file to re-create examples on next startup.\n")
	if err := os.WriteFile(markerPath, markerContent, filePermissions); err != nil {
		slog.Warn("Failed to create examples marker file", "error", err)
	}

	slog.Info("Example souls created successfully")
	return true
}

// hasExistingSouls checks if the directory already contains soul .md files.
func hasExistingSouls(baseDir string) bool {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			return true
		}
	}
	return false
}
