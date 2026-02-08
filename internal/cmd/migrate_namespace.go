package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/persis/filenamespace"
)

const namespaceMigratedMarker = ".namespace-migrated"

// MigrationResult reports what the namespace migration did (or would do in dry-run mode).
type MigrationResult struct {
	DAGFilesMoved       int
	DirEntriesMoved     map[string]int // "dag-runs", "proc", "queue", "suspend", "gitsync"
	ConversationsTagged int
	LogEntriesMoved     int // log directory entries moved
	StatusFilesFixed    int // status.jsonl files with rewritten paths
	AlreadyMigrated     bool // marker file existed
	AlreadyScoped       bool // paths already namespace-scoped
}

func (r *MigrationResult) totalMigrated() int {
	total := r.DAGFilesMoved + r.ConversationsTagged + r.LogEntriesMoved + r.StatusFilesFixed
	for _, n := range r.DirEntriesMoved {
		total += n
	}
	return total
}

// runNamespaceMigration moves existing DAG definitions and run data into the
// default namespace subdirectory ({shortID}). When dryRun is true it counts
// what would be moved without touching the filesystem.
func runNamespaceMigration(paths config.PathsConfig, dryRun bool) (*MigrationResult, error) {
	result := &MigrationResult{
		DirEntriesMoved: make(map[string]int),
	}

	markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)

	if fileExists(markerPath) {
		slog.Debug("namespace migration: already completed, skipping")
		result.AlreadyMigrated = true
		return result, nil
	}

	if isAlreadyNamespaceScoped(paths) {
		slog.Debug("namespace migration: paths already namespace-scoped, skipping")
		result.AlreadyScoped = true
		if !dryRun {
			if err := writeMarker(markerPath); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	defaultShortID := filenamespace.DefaultShortID

	// Move DAG YAML files from root DAGsDir to {DAGsDir}/{defaultShortID}/
	count, err := migrateDAGFiles(paths.DAGsDir, defaultShortID, dryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate DAG files: %w", err)
	}
	result.DAGFilesMoved = count

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
		n, err := moveDirContents(d.srcDir, dstDir, defaultShortID, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate %s: %w", d.name, err)
		}
		if n > 0 {
			result.DirEntriesMoved[d.name] = n
		}
	}

	// Move suspend flags into {DataDir}/{defaultShortID}/suspend/
	if paths.SuspendFlagsDir != "" {
		dstDir := filepath.Join(paths.DataDir, defaultShortID, "suspend")
		n, err := moveDirContents(paths.SuspendFlagsDir, dstDir, "", dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate suspend flags: %w", err)
		}
		if n > 0 {
			result.DirEntriesMoved["suspend"] = n
		}
	}

	// Move git sync state into {DataDir}/{defaultShortID}/gitsync/
	gitSyncDir := filepath.Join(paths.DataDir, "gitsync")
	if fileExists(gitSyncDir) {
		dstDir := filepath.Join(paths.DataDir, defaultShortID, "gitsync")
		n, err := moveDirContents(gitSyncDir, dstDir, "", dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate git sync state: %w", err)
		}
		if n > 0 {
			result.DirEntriesMoved["gitsync"] = n
		}
	}

	// Tag existing agent conversations with the default namespace
	if paths.ConversationsDir != "" {
		n, err := tagConversationsWithNamespace(paths.ConversationsDir, "default", dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to tag conversations: %w", err)
		}
		result.ConversationsTagged = n
	}

	// Move log directories into {LogDir}/{defaultShortID}/
	if paths.LogDir != "" {
		n, err := migrateLogDir(paths.LogDir, paths.AdminLogsDir, paths.NamespacesDir, defaultShortID, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate log directories: %w", err)
		}
		result.LogEntriesMoved = n
	}

	// Fix log paths in status.jsonl files across all namespaces
	if paths.LogDir != "" && paths.DataDir != "" {
		n, err := fixLogPathsInStatusFiles(paths.DataDir, paths.LogDir, defaultShortID, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to fix log paths in status files: %w", err)
		}
		result.StatusFilesFixed = n
	}

	if !dryRun {
		if result.totalMigrated() > 0 {
			slog.Info("namespace migration: data migration to default namespace complete",
				"namespace", "default",
				"short_id", defaultShortID,
			)
		}
		if err := writeMarker(markerPath); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// needsNamespaceMigration reports whether the environment has unmigrated data
// that should be migrated via `dagu migrate namespace`. Fresh installs return
// false so no spurious warning is emitted.
func needsNamespaceMigration(paths config.PathsConfig) (bool, string) {
	markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
	if fileExists(markerPath) {
		return false, ""
	}
	if isAlreadyNamespaceScoped(paths) {
		return false, ""
	}
	if !hasUnmigratedData(paths) {
		return false, "" // fresh install — no warning
	}
	return true, "namespace migration has not been run; execute 'dagu migrate namespace' to migrate existing data"
}

// hasUnmigratedData checks for signs that the environment contains data from
// before namespace scoping: YAML files at the root of DAGsDir or a non-empty
// DAGRunsDir.
func hasUnmigratedData(paths config.PathsConfig) bool {
	// Check for YAML files at root of DAGsDir
	if entries, err := os.ReadDir(paths.DAGsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
				return true
			}
		}
	}

	// Check for entries in DAGRunsDir (excluding the default short ID dir)
	if entries, err := os.ReadDir(paths.DAGRunsDir); err == nil {
		for _, e := range entries {
			if e.Name() != filenamespace.DefaultShortID {
				return true
			}
		}
	}

	// Check for non-namespace log directories in LogDir
	if paths.LogDir != "" {
		adminRel, _ := filepath.Rel(paths.LogDir, paths.AdminLogsDir)
		if entries, err := os.ReadDir(paths.LogDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				if name == adminRel {
					continue
				}
				if len(name) == 4 && isHexString(name) {
					continue
				}
				return true
			}
		}
	}

	return false
}

// runNamespaceMigrationCommand is the CLI handler for `dagu migrate namespace`.
func runNamespaceMigrationCommand(ctx *Context) error {
	dryRun, err := ctx.Command.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}

	skipConfirm, err := ctx.Command.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	// Always scan first to show a preview.
	preview, err := runNamespaceMigration(ctx.Config.Paths, true)
	if err != nil {
		return fmt.Errorf("namespace migration failed: %w", err)
	}

	if preview.AlreadyMigrated {
		logger.Info(ctx, "Namespace migration has already been completed (marker file exists)")
		return nil
	}
	if preview.AlreadyScoped {
		logger.Info(ctx, "Paths are already namespace-scoped, no migration needed")
		return nil
	}

	total := preview.totalMigrated()
	if total == 0 {
		logger.Info(ctx, "No data found to migrate")
		return nil
	}

	// Print preview summary.
	if preview.DAGFilesMoved > 0 {
		logger.Info(ctx, fmt.Sprintf("Would migrate %d DAG file(s)", preview.DAGFilesMoved))
	}
	for name, count := range preview.DirEntriesMoved {
		logger.Info(ctx, fmt.Sprintf("Would migrate %d %s entries", count, name))
	}
	if preview.ConversationsTagged > 0 {
		logger.Info(ctx, fmt.Sprintf("Would migrate %d conversation(s)", preview.ConversationsTagged))
	}
	if preview.LogEntriesMoved > 0 {
		logger.Info(ctx, fmt.Sprintf("Would migrate %d log directory entries", preview.LogEntriesMoved))
	}
	if preview.StatusFilesFixed > 0 {
		logger.Info(ctx, fmt.Sprintf("Would fix paths in %d status file(s)", preview.StatusFilesFixed))
	}

	if dryRun {
		logger.Info(ctx, "Re-run without --dry-run to apply changes")
		return nil
	}

	// Confirmation gate.
	if !skipConfirm {
		if !confirmAction("Continue?") {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Execute for real.
	result, err := runNamespaceMigration(ctx.Config.Paths, false)
	if err != nil {
		return fmt.Errorf("namespace migration failed: %w", err)
	}

	if result.DAGFilesMoved > 0 {
		logger.Info(ctx, fmt.Sprintf("Migrated %d DAG file(s)", result.DAGFilesMoved))
	}
	for name, count := range result.DirEntriesMoved {
		logger.Info(ctx, fmt.Sprintf("Migrated %d %s entries", count, name))
	}
	if result.ConversationsTagged > 0 {
		logger.Info(ctx, fmt.Sprintf("Migrated %d conversation(s)", result.ConversationsTagged))
	}
	if result.LogEntriesMoved > 0 {
		logger.Info(ctx, fmt.Sprintf("Migrated %d log directory entries", result.LogEntriesMoved))
	}
	if result.StatusFilesFixed > 0 {
		logger.Info(ctx, fmt.Sprintf("Fixed paths in %d status file(s)", result.StatusFilesFixed))
	}

	return nil
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
// When dryRun is true it counts files without moving them.
func migrateDAGFiles(dagsDir, shortID string, dryRun bool) (int, error) {
	entries, err := os.ReadDir(dagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read DAGs directory: %w", err)
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
		return 0, nil
	}

	if dryRun {
		return len(yamlFiles), nil
	}

	dstDir := filepath.Join(dagsDir, shortID)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return 0, fmt.Errorf("failed to create namespace DAGs directory: %w", err)
	}

	slog.Info("namespace migration: moving DAG definitions into default namespace",
		"count", len(yamlFiles),
		"destination", dstDir,
	)

	for _, entry := range yamlFiles {
		src := filepath.Join(dagsDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		if err := os.Rename(src, dst); err != nil {
			return 0, fmt.Errorf("failed to move DAG file %s: %w", entry.Name(), err)
		}
		slog.Debug("namespace migration: moved DAG file", "file", entry.Name())
	}

	return len(yamlFiles), nil
}

// moveDirContents moves the contents of srcDir into dstDir.
// skipEntry is the name of a subdirectory to skip (e.g., the namespace shortID
// to avoid moving the destination into itself). Pass "" to skip nothing.
// When dryRun is true it counts entries without moving them.
func moveDirContents(srcDir, dstDir, skipEntry string, dryRun bool) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory %s: %w", srcDir, err)
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
		return 0, nil
	}

	if dryRun {
		return len(toMove), nil
	}

	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return 0, fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	slog.Info("namespace migration: moving data into default namespace",
		"source", srcDir,
		"destination", dstDir,
		"count", len(toMove),
	)

	for _, entry := range toMove {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		if err := os.Rename(src, dst); err != nil {
			return 0, fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
		slog.Debug("namespace migration: moved entry", "name", entry.Name())
	}

	return len(toMove), nil
}

// tagConversationsWithNamespace scans all conversation JSON files and sets
// the namespace field to the given value if it is empty. When dryRun is true
// it counts files that would be tagged without modifying them.
func tagConversationsWithNamespace(conversationsDir, namespace string, dryRun bool) (int, error) {
	// Conversations are stored as {conversationsDir}/{userID}/{conversationID}.json
	userDirs, err := os.ReadDir(conversationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	tagged := 0
	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}

		userPath := filepath.Join(conversationsDir, userDir.Name())
		convFiles, err := os.ReadDir(userPath)
		if err != nil {
			slog.Warn("namespace migration: failed to read user conversation directory",
				"user_dir", userDir.Name(), "error", err)
			continue
		}

		for _, convFile := range convFiles {
			if convFile.IsDir() || !strings.HasSuffix(convFile.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(userPath, convFile.Name())
			if dryRun {
				needs, err := conversationNeedsTag(filePath, namespace)
				if err != nil {
					slog.Warn("namespace migration: failed to check conversation",
						"file", filePath, "error", err)
				} else if needs {
					tagged++
				}
			} else {
				if t, err := tagConversationFile(filePath, namespace); err != nil {
					slog.Warn("namespace migration: failed to tag conversation",
						"file", filePath, "error", err)
				} else if t {
					tagged++
				}
			}
		}
	}

	if !dryRun && tagged > 0 {
		slog.Info("namespace migration: tagged agent conversations with default namespace",
			"count", tagged,
			"namespace", namespace,
		)
	}

	return tagged, nil
}

// conversationNeedsTag checks whether a conversation JSON file needs a namespace tag
// without modifying it.
func conversationNeedsTag(filePath, _ string) (bool, error) {
	data, err := os.ReadFile(filePath) // #nosec G304 - path constructed from internal baseDir
	if err != nil {
		return false, err
	}

	var conv map[string]json.RawMessage
	if err := json.Unmarshal(data, &conv); err != nil {
		return false, err
	}

	if ns, ok := conv["namespace"]; ok {
		var existing string
		if err := json.Unmarshal(ns, &existing); err == nil && existing != "" {
			return false, nil
		}
	}

	return true, nil
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

// migrateLogDir moves DAG log directories from the root of logDir into
// {logDir}/{defaultShortID}/. It skips the admin logs directory and any
// directory whose name matches a registered namespace short ID.
// When dryRun is true it counts entries without moving them.
func migrateLogDir(logDir, adminLogsDir, namespacesDir, defaultShortID string, dryRun bool) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read log directory: %w", err)
	}

	// Build skip set: admin dir + registered namespace dirs
	skip := make(map[string]bool)

	adminRel, relErr := filepath.Rel(logDir, adminLogsDir)
	if relErr == nil && !strings.Contains(adminRel, string(filepath.Separator)) {
		skip[adminRel] = true
	}

	if nsEntries, nsErr := os.ReadDir(namespacesDir); nsErr == nil {
		for _, e := range nsEntries {
			name := e.Name()
			if strings.HasSuffix(name, ".json") {
				skip[strings.TrimSuffix(name, ".json")] = true
			}
		}
	}

	var toMove []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if skip[e.Name()] {
			continue
		}
		toMove = append(toMove, e)
	}

	if len(toMove) == 0 {
		return 0, nil
	}

	if dryRun {
		return len(toMove), nil
	}

	dstDir := filepath.Join(logDir, defaultShortID)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return 0, fmt.Errorf("failed to create namespace log directory: %w", err)
	}

	slog.Info("namespace migration: moving log directories into default namespace",
		"count", len(toMove),
		"destination", dstDir,
	)

	for _, e := range toMove {
		src := filepath.Join(logDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			return 0, fmt.Errorf("failed to move log directory %s: %w", e.Name(), err)
		}
		slog.Debug("namespace migration: moved log directory", "name", e.Name())
	}

	return len(toMove), nil
}

// fixLogPathsInStatusFiles rewrites log file paths in status.jsonl files so
// they point to the namespace-scoped log directory. It uses a 3-step safe
// replacement to avoid double-scoping paths that are already correct.
// When dryRun is true it counts files that would be fixed without modifying them.
func fixLogPathsInStatusFiles(dataDir, logDir, defaultShortID string, dryRun bool) (int, error) {
	oldPrefix := logDir + "/"
	newPrefix := filepath.Join(logDir, defaultShortID) + "/"
	placeholder := "\x00NS\x00"

	fixed := 0

	err := filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if d.IsDir() || d.Name() != "status.jsonl" {
			return nil
		}

		content, readErr := os.ReadFile(path) // #nosec G304 - path from internal walk
		if readErr != nil {
			slog.Warn("namespace migration: failed to read status file", "path", path, "error", readErr)
			return nil
		}

		s := string(content)
		if !strings.Contains(s, oldPrefix) {
			return nil
		}

		// 3-step safe replacement:
		// 1. Protect already-correct paths
		s = strings.ReplaceAll(s, newPrefix, placeholder)
		// 2. Fix old paths
		s = strings.ReplaceAll(s, oldPrefix, newPrefix)
		// 3. Restore protected paths
		s = strings.ReplaceAll(s, placeholder, newPrefix)

		if s == string(content) {
			return nil // no actual changes
		}

		fixed++

		if dryRun {
			return nil
		}

		// Atomic write: temp file + rename
		dir := filepath.Dir(path)
		tmp, tmpErr := os.CreateTemp(dir, "status-*.jsonl.tmp")
		if tmpErr != nil {
			return fmt.Errorf("failed to create temp file for %s: %w", path, tmpErr)
		}
		tmpName := tmp.Name()

		if _, wErr := tmp.WriteString(s); wErr != nil {
			tmp.Close()
			os.Remove(tmpName)
			return fmt.Errorf("failed to write temp file for %s: %w", path, wErr)
		}
		if cErr := tmp.Close(); cErr != nil {
			os.Remove(tmpName)
			return fmt.Errorf("failed to close temp file for %s: %w", path, cErr)
		}
		if rErr := os.Rename(tmpName, path); rErr != nil {
			os.Remove(tmpName)
			return fmt.Errorf("failed to rename temp file for %s: %w", path, rErr)
		}

		slog.Debug("namespace migration: fixed log paths in status file", "path", path)
		return nil
	})

	return fixed, err
}

// isHexString returns true if s is a non-empty string of lowercase hex characters.
func isHexString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
