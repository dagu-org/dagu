package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/persis/filenamespace"
)

const namespaceMigratedMarker = ".namespace-migrated"

// migrateToDefaultNamespace moves existing DAG definitions and run data
// into the default namespace subdirectory ({shortID}). The migration is
// idempotent: a marker file prevents re-execution on subsequent startups.
// Environments with already namespace-scoped paths (e.g., test setups) are
// detected and skipped automatically.
func migrateToDefaultNamespace(paths config.PathsConfig) error {
	markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)

	if fileExists(markerPath) {
		slog.Debug("namespace migration: already completed, skipping")
		return nil
	}

	// If paths are already namespace-scoped (e.g., DAGRunsDir is {DataDir}/0000/dag-runs),
	// write the marker and skip — no migration needed.
	if isAlreadyNamespaceScoped(paths) {
		slog.Debug("namespace migration: paths already namespace-scoped, skipping")
		return writeMarker(markerPath)
	}

	defaultShortID := filenamespace.DefaultShortID
	migrated := false

	// Move DAG YAML files from root DAGsDir to {DAGsDir}/{defaultShortID}/
	moved, err := migrateDAGFiles(paths.DAGsDir, defaultShortID)
	if err != nil {
		return fmt.Errorf("failed to migrate DAG files: %w", err)
	}
	migrated = migrated || moved

	// Move run data directories into {DataDir}/{defaultShortID}/
	dataDirs := []struct {
		name   string
		srcDir string
	}{
		{"dag-runs", paths.DAGRunsDir},
		{"proc", paths.ProcDir},
		{"queue", paths.QueueDir},
	}

	for _, d := range dataDirs {
		dstDir := filepath.Join(paths.DataDir, defaultShortID, d.name)
		m, err := moveDirContents(d.srcDir, dstDir, defaultShortID)
		if err != nil {
			return fmt.Errorf("failed to migrate %s: %w", d.name, err)
		}
		migrated = migrated || m
	}

	// Move suspend flags into {DataDir}/{defaultShortID}/suspend/
	if paths.SuspendFlagsDir != "" {
		dstDir := filepath.Join(paths.DataDir, defaultShortID, "suspend")
		m, err := moveDirContents(paths.SuspendFlagsDir, dstDir, "")
		if err != nil {
			return fmt.Errorf("failed to migrate suspend flags: %w", err)
		}
		migrated = migrated || m
	}

	// Move git sync state into {DataDir}/{defaultShortID}/gitsync/
	gitSyncDir := filepath.Join(paths.DataDir, "gitsync")
	if fileExists(gitSyncDir) {
		dstDir := filepath.Join(paths.DataDir, defaultShortID, "gitsync")
		m, err := moveDirContents(gitSyncDir, dstDir, "")
		if err != nil {
			return fmt.Errorf("failed to migrate git sync state: %w", err)
		}
		migrated = migrated || m
	}

	// Tag existing agent conversations with the default namespace
	if paths.ConversationsDir != "" {
		tagged, err := tagConversationsWithNamespace(paths.ConversationsDir, "default")
		if err != nil {
			return fmt.Errorf("failed to tag conversations: %w", err)
		}
		migrated = migrated || tagged
	}

	if migrated {
		slog.Info("auto-migration: data migration to default namespace complete",
			"namespace", "default",
			"short_id", defaultShortID,
		)
	}

	return writeMarker(markerPath)
}

// isAlreadyNamespaceScoped checks whether the configured paths already point to
// namespace-scoped directories (e.g., {DataDir}/0000/dag-runs instead of {DataDir}/dag-runs).
func isAlreadyNamespaceScoped(paths config.PathsConfig) bool {
	rel, err := filepath.Rel(paths.DataDir, paths.DAGRunsDir)
	if err != nil {
		return false
	}
	// Non-namespaced: rel = "dag-runs" (1 part)
	// Namespace-scoped: rel = "0000/dag-runs" (2 parts)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) >= 2 && parts[len(parts)-1] == "dag-runs"
}

// writeMarker creates the migration marker file to prevent re-migration.
func writeMarker(markerPath string) error {
	if err := os.MkdirAll(filepath.Dir(markerPath), 0750); err != nil {
		return fmt.Errorf("failed to create directory for migration marker: %w", err)
	}
	if err := os.WriteFile(markerPath, []byte("migrated\n"), 0600); err != nil {
		return fmt.Errorf("failed to write migration marker: %w", err)
	}
	return nil
}

// migrateDAGFiles moves YAML files from the root DAGsDir to a namespace subdirectory.
// Returns true if any files were moved.
func migrateDAGFiles(dagsDir, shortID string) (bool, error) {
	entries, err := os.ReadDir(dagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read DAGs directory: %w", err)
	}

	// Collect YAML files at root level
	var yamlFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			yamlFiles = append(yamlFiles, entry)
		}
	}

	if len(yamlFiles) == 0 {
		return false, nil
	}

	dstDir := filepath.Join(dagsDir, shortID)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return false, fmt.Errorf("failed to create namespace DAGs directory: %w", err)
	}

	slog.Info("auto-migration: moving DAG definitions into default namespace",
		"count", len(yamlFiles),
		"destination", dstDir,
	)

	for _, entry := range yamlFiles {
		src := filepath.Join(dagsDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		if err := os.Rename(src, dst); err != nil {
			return false, fmt.Errorf("failed to move DAG file %s: %w", entry.Name(), err)
		}
		slog.Debug("auto-migration: moved DAG file", "file", entry.Name())
	}

	return true, nil
}

// moveDirContents moves the contents of srcDir into dstDir.
// skipEntry is the name of a subdirectory to skip (e.g., the namespace shortID
// to avoid moving the destination into itself). Pass "" to skip nothing.
// Returns true if any entries were moved.
func moveDirContents(srcDir, dstDir, skipEntry string) (bool, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read directory %s: %w", srcDir, err)
	}

	// Filter out entries to skip
	var toMove []os.DirEntry
	for _, entry := range entries {
		if skipEntry != "" && entry.Name() == skipEntry {
			continue
		}
		toMove = append(toMove, entry)
	}

	if len(toMove) == 0 {
		return false, nil
	}

	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return false, fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	slog.Info("auto-migration: moving data into default namespace",
		"source", srcDir,
		"destination", dstDir,
		"count", len(toMove),
	)

	for _, entry := range toMove {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		if err := os.Rename(src, dst); err != nil {
			return false, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
		slog.Debug("auto-migration: moved entry", "name", entry.Name())
	}

	return true, nil
}

// tagConversationsWithNamespace scans all conversation JSON files and sets
// the namespace field to the given value if it is empty. Returns true if
// any conversations were tagged.
func tagConversationsWithNamespace(conversationsDir, namespace string) (bool, error) {
	// Conversations are stored as {conversationsDir}/{userID}/{conversationID}.json
	userDirs, err := os.ReadDir(conversationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	tagged := 0
	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}

		userPath := filepath.Join(conversationsDir, userDir.Name())
		convFiles, err := os.ReadDir(userPath)
		if err != nil {
			slog.Warn("auto-migration: failed to read user conversation directory",
				"user_dir", userDir.Name(), "error", err)
			continue
		}

		for _, convFile := range convFiles {
			if convFile.IsDir() || !strings.HasSuffix(convFile.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(userPath, convFile.Name())
			if t, err := tagConversationFile(filePath, namespace); err != nil {
				slog.Warn("auto-migration: failed to tag conversation",
					"file", filePath, "error", err)
			} else if t {
				tagged++
			}
		}
	}

	if tagged > 0 {
		slog.Info("auto-migration: tagged agent conversations with default namespace",
			"count", tagged,
			"namespace", namespace,
		)
	}

	return tagged > 0, nil
}

// tagConversationFile reads a conversation JSON file, sets the namespace if
// empty, and writes it back. Returns true if the file was modified.
func tagConversationFile(filePath, namespace string) (bool, error) {
	data, err := os.ReadFile(filePath) // #nosec G304 - path constructed from internal baseDir
	if err != nil {
		return false, err
	}

	// Parse as a generic map to preserve all fields
	var conv map[string]json.RawMessage
	if err := json.Unmarshal(data, &conv); err != nil {
		return false, err
	}

	// Check if namespace is already set
	if ns, ok := conv["namespace"]; ok {
		var existing string
		if err := json.Unmarshal(ns, &existing); err == nil && existing != "" {
			return false, nil // Already tagged
		}
	}

	// Set the namespace
	nsJSON, _ := json.Marshal(namespace)
	conv["namespace"] = nsJSON

	updated, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return false, err
	}

	if err := os.WriteFile(filePath, updated, 0600); err != nil {
		return false, err
	}

	return true, nil
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
