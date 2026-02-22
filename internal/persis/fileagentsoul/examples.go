package fileagentsoul

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
)

//go:embed examples/*.md
var exampleSoulsFS embed.FS

const examplesMarkerFile = ".examples-created"

// ExampleSoulIDs holds the IDs of bundled example souls, derived from the embedded filesystem.
var ExampleSoulIDs []string

func init() {
	entries, err := exampleSoulsFS.ReadDir("examples")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		ExampleSoulIDs = append(ExampleSoulIDs, id)
	}
}

// SeedExampleSouls writes bundled example souls to baseDir if not already seeded.
// Returns true if examples were created this call.
func SeedExampleSouls(ctx context.Context, baseDir string) bool {
	markerPath := filepath.Join(baseDir, examplesMarkerFile)
	if _, err := os.Stat(markerPath); err == nil {
		return false // already seeded
	}
	if hasExistingSouls(baseDir) {
		return false
	}

	if err := os.MkdirAll(baseDir, soulDirPermissions); err != nil {
		logger.Warn(ctx, "Failed to create souls directory", slog.String("dir", baseDir), tag.Error(err))
		return false
	}

	logger.Info(ctx, "Creating example souls for first-time users", slog.String("dir", baseDir))

	entries, err := exampleSoulsFS.ReadDir("examples")
	if err != nil {
		logger.Warn(ctx, "Failed to read embedded example souls", tag.Error(err))
		return false
	}

	created := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, readErr := exampleSoulsFS.ReadFile("examples/" + entry.Name())
		if readErr != nil {
			logger.Warn(ctx, "Failed to read embedded example soul", slog.String("name", entry.Name()), tag.Error(readErr))
			continue
		}

		destPath := filepath.Join(baseDir, entry.Name())
		if _, statErr := os.Stat(destPath); statErr == nil {
			continue // don't overwrite existing files
		}

		if err := os.WriteFile(destPath, data, filePermissions); err != nil {
			logger.Warn(ctx, "Failed to write example soul", slog.String("path", destPath), tag.Error(err))
		} else {
			created++
		}
	}

	if created == 0 {
		return false
	}

	markerContent := []byte("# This file indicates that example souls have been created.\n# Delete this file to re-create examples on next startup.\n")
	if err := os.WriteFile(markerPath, markerContent, filePermissions); err != nil {
		logger.Warn(ctx, "Failed to create examples marker file", tag.Error(err))
		return false
	}

	logger.Info(ctx, "Example souls created successfully")
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
