// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileagentskill

import (
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:examples
var exampleSkillsFS embed.FS

const examplesMarkerFile = ".examples-created"

// ExampleSkillIDs returns the IDs of bundled example skills.
func ExampleSkillIDs() []string {
	return []string{"dagu-ai-workflows", "dagu-containers", "dagu-server-worker"}
}

// SkillFS returns the embedded example skills filesystem.
func SkillFS() embed.FS {
	return exampleSkillsFS
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

	seedableIDs := make(map[string]struct{}, len(ExampleSkillIDs()))
	for _, id := range ExampleSkillIDs() {
		seedableIDs[id] = struct{}{}
	}

	err := fs.WalkDir(exampleSkillsFS, "examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// path is "examples/{skill-id}/SKILL.md"
		relPath := strings.TrimPrefix(path, "examples/")
		topDir := strings.SplitN(relPath, "/", 2)[0]
		if _, ok := seedableIDs[topDir]; !ok {
			return nil // skip non-example skills (e.g. dagu/ used by ai install)
		}
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

const builtinKnowledgeEmbedDir = "examples/dagu/references"

// SeedReferences extracts built-in reference documents to the given directory.
// These are read-only knowledge files the AI agent can read on demand.
// Returns the directory path if successful, empty string on failure.
// Files are always overwritten on each startup to keep them up-to-date with the binary.
func SeedReferences(destDir string) string {
	if err := os.MkdirAll(destDir, skillDirPermissions); err != nil {
		slog.Warn("Failed to create builtin knowledge directory", "dir", destDir, "error", err)
		return ""
	}

	err := fs.WalkDir(exampleSkillsFS, builtinKnowledgeEmbedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := strings.TrimPrefix(path, builtinKnowledgeEmbedDir+"/")
		destPath := filepath.Join(destDir, relPath)

		data, readErr := exampleSkillsFS.ReadFile(path)
		if readErr != nil {
			slog.Warn("Failed to read embedded knowledge file", "path", path, "error", readErr)
			return nil
		}

		if err := os.WriteFile(destPath, data, filePermissions); err != nil {
			slog.Warn("Failed to write knowledge file", "path", destPath, "error", err)
		}
		return nil
	})
	if err != nil {
		slog.Warn("Failed to walk embedded knowledge files", "error", err)
		return ""
	}

	return destDir
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
