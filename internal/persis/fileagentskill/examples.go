package fileagentskill

import (
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//go:embed examples/*/SKILL.md
var exampleSkillsFS embed.FS

const examplesMarkerFile = ".examples-created"

// ExampleSkillIDs returns the IDs of bundled example skills.
func ExampleSkillIDs() []string {
	return []string{"dagu-ai-workflows", "dagu-containers", "dagu-server-worker"}
}

// SeedExampleSkills writes bundled example skills to baseDir if not already seeded.
// Returns true if examples were created this call.
func SeedExampleSkills(baseDir string) bool {
	markerPath := filepath.Join(baseDir, examplesMarkerFile)
	if _, err := os.Stat(markerPath); err == nil {
		return false // already seeded
	}
	if hasExistingSkills(baseDir) {
		return false
	}

	if err := os.MkdirAll(baseDir, skillDirPermissions); err != nil {
		slog.Warn("Failed to create skills directory", "dir", baseDir, "error", err)
		return false
	}

	slog.Info("Creating example skills for first-time users", "dir", baseDir)

	err := fs.WalkDir(exampleSkillsFS, "examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// path is "examples/{skill-id}/SKILL.md"
		relPath := strings.TrimPrefix(path, "examples/")
		destPath := filepath.Join(baseDir, relPath)

		data, readErr := exampleSkillsFS.ReadFile(path)
		if readErr != nil {
			slog.Warn("Failed to read embedded example skill", "path", path, "error", readErr)
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), skillDirPermissions); err != nil {
			slog.Warn("Failed to create example skill directory", "path", destPath, "error", err)
			return nil
		}

		if _, statErr := os.Stat(destPath); statErr == nil {
			return nil // don't overwrite existing files
		}

		if err := os.WriteFile(destPath, data, filePermissions); err != nil {
			slog.Warn("Failed to write example skill", "path", destPath, "error", err)
		}
		return nil
	})
	if err != nil {
		slog.Warn("Failed to walk embedded example skills", "error", err)
		return false
	}

	markerContent := []byte("# This file indicates that example skills have been created.\n# Delete this file to re-create examples on next startup.\n")
	if err := os.WriteFile(markerPath, markerContent, filePermissions); err != nil {
		slog.Warn("Failed to create examples marker file", "error", err)
	}

	slog.Info("Example skills created successfully")
	return true
}

// hasExistingSkills checks if the directory already contains skill subdirectories.
func hasExistingSkills(baseDir string) bool {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(baseDir, entry.Name(), skillFilename)
		if _, err := os.Stat(skillPath); err == nil {
			return true
		}
	}
	return false
}
