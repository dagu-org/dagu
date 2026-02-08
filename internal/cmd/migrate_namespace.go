package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filenamespace"
)

const namespaceMigratedMarker = ".namespace-migrated"

// MigrationResult reports what the namespace migration did (or would do in dry-run mode).
type MigrationResult struct {
	DAGFilesMoved       int
	DirEntriesMoved     map[string]int // "dag-runs", "proc", "queue", "suspend", "gitsync"
	ConversationsTagged int
	LogEntriesMoved     int  // log directory entries moved
	StatusFilesFixed    int  // status.jsonl files with rewritten paths
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
// default namespace subdirectory ({id}). When dryRun is true it counts
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

	defaultID := filenamespace.DefaultID

	// Move DAG YAML files from root DAGsDir to {DAGsDir}/{defaultID}/
	count, err := migrateDAGFiles(paths.DAGsDir, defaultID, dryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate DAG files: %w", err)
	}
	result.DAGFilesMoved = count

	// Move run data directories into {DataDir}/{defaultID}/
	dataDirs := []struct {
		name   string
		srcDir string
	}{
		{"dag-runs", paths.DAGRunsDir},
		{"proc", paths.ProcDir},
		{"queue", paths.QueueDir},
	}

	for _, d := range dataDirs {
		dstDir := filepath.Join(exec.NamespaceDir(paths.DataDir, defaultID), d.name)
		n, err := moveDirContents(d.srcDir, dstDir, "", dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate %s: %w", d.name, err)
		}
		if n > 0 {
			result.DirEntriesMoved[d.name] = n
		}
	}

	// Move suspend flags into {DataDir}/ns/{defaultID}/suspend/
	if paths.SuspendFlagsDir != "" {
		dstDir := filepath.Join(exec.NamespaceDir(paths.DataDir, defaultID), "suspend")
		n, err := moveDirContents(paths.SuspendFlagsDir, dstDir, "", dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate suspend flags: %w", err)
		}
		if n > 0 {
			result.DirEntriesMoved["suspend"] = n
		}
	}

	// Move git sync state into {DataDir}/ns/{defaultID}/gitsync/
	gitSyncDir := filepath.Join(paths.DataDir, "gitsync")
	if fileExists(gitSyncDir) {
		dstDir := filepath.Join(exec.NamespaceDir(paths.DataDir, defaultID), "gitsync")
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

	// Move log directories into {LogDir}/{defaultID}/
	if paths.LogDir != "" {
		n, err := migrateLogDir(paths.LogDir, paths.AdminLogsDir, paths.NamespacesDir, defaultID, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate log directories: %w", err)
		}
		result.LogEntriesMoved = n
	}

	// Fix log paths in status.jsonl files across all namespaces
	if paths.LogDir != "" && paths.DataDir != "" {
		n, err := fixLogPathsInStatusFiles(paths.DataDir, paths.LogDir, defaultID, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to fix log paths in status files: %w", err)
		}
		result.StatusFilesFixed = n
	}

	if !dryRun {
		if result.totalMigrated() > 0 {
			slog.Info("namespace migration: data migration to default namespace complete",
				"namespace", "default",
				"id", defaultID,
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
	// Check v1 migration
	v1MarkerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
	v1Done := fileExists(v1MarkerPath) || isAlreadyNamespaceScoped(paths)

	if !v1Done && hasUnmigratedData(paths) {
		return true, "namespace migration has not been run; execute 'dagu migrate namespace' to migrate existing data"
	}

	// Check v2 ns/ relayout
	v2MarkerPath := filepath.Join(paths.DataDir, namespaceNsMigratedMarker)
	if !fileExists(v2MarkerPath) && hasUnrelocatedNamespaceDirs(paths) {
		return true, "namespace ns/ relayout has not been run; execute 'dagu migrate namespace' to relocate namespace directories"
	}

	return false, ""
}

// hasUnrelocatedNamespaceDirs checks whether namespace ID directories exist
// directly under base dirs (not under ns/), indicating v2 migration is needed.
func hasUnrelocatedNamespaceDirs(paths config.PathsConfig) bool {
	nsIDs := collectNamespaceIDs(paths.NamespacesDir)
	if len(nsIDs) == 0 {
		return false
	}

	for _, baseDir := range []string{paths.DataDir, paths.LogDir} {
		if baseDir == "" {
			continue
		}
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && nsIDs[e.Name()] {
				return true
			}
		}
	}
	return false
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
			if e.Name() != filenamespace.DefaultID {
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
				if name == adminRel || name == exec.NsDir {
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
// It runs two phases: v1 moves flat data into namespace dirs, v2 moves
// namespace dirs under the ns/ parent.
func runNamespaceMigrationCommand(ctx *Context) error {
	dryRun, err := ctx.Command.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}

	skipConfirm, err := ctx.Command.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	// --- Phase 1: v1 migration (flat → namespace dirs) ---
	v1Preview, err := runNamespaceMigration(ctx.Config.Paths, true)
	if err != nil {
		return fmt.Errorf("namespace migration v1 scan failed: %w", err)
	}

	v1Skip := v1Preview.AlreadyMigrated || v1Preview.AlreadyScoped
	v1Total := v1Preview.totalMigrated()

	// --- Phase 2: v2 ns/ relayout ---
	v2Preview, err := runNamespaceNsRelayout(ctx.Config.Paths, true)
	if err != nil {
		return fmt.Errorf("namespace ns-relayout scan failed: %w", err)
	}

	v2Total := v2Preview.totalMigrated()

	if v1Skip && v2Preview.AlreadyMigrated {
		logger.Info(ctx, "All namespace migrations have already been completed")
		return nil
	}

	if v1Total == 0 && v2Total == 0 {
		// No work to do — write markers and return.
		if !dryRun {
			if !v1Preview.AlreadyMigrated && !v1Preview.AlreadyScoped {
				_ = writeMarker(filepath.Join(ctx.Config.Paths.DataDir, namespaceMigratedMarker))
			}
			if !v2Preview.AlreadyMigrated {
				_ = writeMarker(filepath.Join(ctx.Config.Paths.DataDir, namespaceNsMigratedMarker))
			}
		}
		logger.Info(ctx, "No data found to migrate")
		return nil
	}

	// Print preview summary.
	if !v1Skip && v1Total > 0 {
		logger.Info(ctx, "--- Phase 1: Namespace migration ---")
		if v1Preview.DAGFilesMoved > 0 {
			logger.Info(ctx, fmt.Sprintf("Would migrate %d DAG file(s)", v1Preview.DAGFilesMoved))
		}
		for name, count := range v1Preview.DirEntriesMoved {
			logger.Info(ctx, fmt.Sprintf("Would migrate %d %s entries", count, name))
		}
		if v1Preview.ConversationsTagged > 0 {
			logger.Info(ctx, fmt.Sprintf("Would migrate %d conversation(s)", v1Preview.ConversationsTagged))
		}
		if v1Preview.LogEntriesMoved > 0 {
			logger.Info(ctx, fmt.Sprintf("Would migrate %d log directory entries", v1Preview.LogEntriesMoved))
		}
		if v1Preview.StatusFilesFixed > 0 {
			logger.Info(ctx, fmt.Sprintf("Would fix paths in %d status file(s)", v1Preview.StatusFilesFixed))
		}
	}

	if !v2Preview.AlreadyMigrated && v2Total > 0 {
		logger.Info(ctx, "--- Phase 2: ns/ directory relayout ---")
		if v2Preview.DirsRelocated > 0 {
			logger.Info(ctx, fmt.Sprintf("Would relocate %d namespace directory(ies) under ns/", v2Preview.DirsRelocated))
		}
		if v2Preview.StatusFilesFixed > 0 {
			logger.Info(ctx, fmt.Sprintf("Would fix log paths in %d status file(s)", v2Preview.StatusFilesFixed))
		}
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

	// Execute v1 for real.
	if !v1Skip && v1Total > 0 {
		result, v1Err := runNamespaceMigration(ctx.Config.Paths, false)
		if v1Err != nil {
			return fmt.Errorf("namespace migration failed: %w", v1Err)
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
	}

	// Execute v2 for real.
	if !v2Preview.AlreadyMigrated {
		v2Result, v2Err := runNamespaceNsRelayout(ctx.Config.Paths, false)
		if v2Err != nil {
			return fmt.Errorf("namespace ns-relayout failed: %w", v2Err)
		}
		if v2Result.DirsRelocated > 0 {
			logger.Info(ctx, fmt.Sprintf("Relocated %d namespace directory(ies) under ns/", v2Result.DirsRelocated))
		}
		if v2Result.StatusFilesFixed > 0 {
			logger.Info(ctx, fmt.Sprintf("Fixed log paths in %d status file(s)", v2Result.StatusFilesFixed))
		}
	}

	return nil
}

// isAlreadyNamespaceScoped checks whether the configured paths already point to
// namespace-scoped directories (e.g., {DataDir}/ns/0000/dag-runs instead of {DataDir}/dag-runs).
func isAlreadyNamespaceScoped(paths config.PathsConfig) bool {
	rel, err := filepath.Rel(paths.DataDir, paths.DAGRunsDir)
	if err != nil {
		return false
	}
	// Non-namespaced: rel = "dag-runs" (1 part)
	// Namespace-scoped: rel = "ns/0000/dag-runs" (3 parts, first is "ns")
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) >= 3 && parts[0] == exec.NsDir && parts[len(parts)-1] == "dag-runs"
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
func migrateDAGFiles(dagsDir, id string, dryRun bool) (int, error) {
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

	dstDir := filepath.Join(dagsDir, id)
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
// skipEntry is the name of a subdirectory to skip (e.g., the namespace ID
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
// {logDir}/{defaultID}/. It skips the admin logs directory and any
// directory whose name matches a registered namespace ID.
// When dryRun is true it counts entries without moving them.
func migrateLogDir(logDir, adminLogsDir, namespacesDir, defaultID string, dryRun bool) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read log directory: %w", err)
	}

	// Build skip set: admin dir + ns dir + registered namespace dirs
	skip := make(map[string]bool)
	skip[exec.NsDir] = true

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

	dstDir := exec.NamespaceDir(logDir, defaultID)
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
func fixLogPathsInStatusFiles(dataDir, logDir, defaultID string, dryRun bool) (int, error) {
	oldPrefix := logDir + "/"
	newPrefix := exec.NamespaceDir(logDir, defaultID) + "/"
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
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to write temp file for %s: %w", path, wErr)
		}
		if cErr := tmp.Close(); cErr != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to close temp file for %s: %w", path, cErr)
		}
		if rErr := os.Rename(tmpName, path); rErr != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to rename temp file for %s: %w", path, rErr)
		}

		slog.Debug("namespace migration: fixed log paths in status file", "path", path)
		return nil
	})

	return fixed, err
}

const namespaceNsMigratedMarker = ".namespace-ns-migrated"

// NsRelayoutResult reports what the ns/ relayout migration did.
type NsRelayoutResult struct {
	DirsRelocated    int
	StatusFilesFixed int
	AlreadyMigrated  bool
}

func (r *NsRelayoutResult) totalMigrated() int {
	return r.DirsRelocated + r.StatusFilesFixed
}

// runNamespaceNsRelayout moves namespace directories from {base}/{nsID}/ to
// {base}/ns/{nsID}/ for users who already ran the v1 migration.
// When dryRun is true it counts what would be moved without touching the filesystem.
func runNamespaceNsRelayout(paths config.PathsConfig, dryRun bool) (*NsRelayoutResult, error) {
	result := &NsRelayoutResult{}

	markerPath := filepath.Join(paths.DataDir, namespaceNsMigratedMarker)
	if fileExists(markerPath) {
		result.AlreadyMigrated = true
		return result, nil
	}

	// Collect known namespace IDs from the registry
	nsIDs := collectNamespaceIDs(paths.NamespacesDir)
	if len(nsIDs) == 0 {
		// No registered namespaces — nothing to relocate.
		if !dryRun {
			if err := writeMarker(markerPath); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	// Relocate namespace dirs under each base directory (not DAGsDir — DAGs stay flat)
	baseDirs := []string{paths.DataDir}
	if paths.LogDir != "" {
		baseDirs = append(baseDirs, paths.LogDir)
	}

	for _, baseDir := range baseDirs {
		n, err := relocateNamespaceDirs(baseDir, nsIDs, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to relocate namespace dirs in %s: %w", baseDir, err)
		}
		result.DirsRelocated += n
	}

	// Fix log paths in status.jsonl files: /{nsID}/ → /ns/{nsID}/
	if paths.LogDir != "" && paths.DataDir != "" {
		n, err := fixNsRelayoutLogPaths(paths.DataDir, paths.LogDir, nsIDs, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to fix log paths for ns relayout: %w", err)
		}
		result.StatusFilesFixed = n
	}

	if !dryRun {
		if err := writeMarker(markerPath); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// collectNamespaceIDs reads the namespace registry and returns a set of known
// namespace IDs (the 4-char hex short IDs).
func collectNamespaceIDs(namespacesDir string) map[string]bool {
	ids := make(map[string]bool)
	entries, err := os.ReadDir(namespacesDir)
	if err != nil {
		return ids
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			id := strings.TrimSuffix(name, ".json")
			if len(id) == 4 && isHexString(id) {
				ids[id] = true
			}
		}
	}
	return ids
}

// relocateNamespaceDirs moves directories matching known namespace IDs from
// {baseDir}/{nsID}/ to {baseDir}/ns/{nsID}/.
func relocateNamespaceDirs(baseDir string, nsIDs map[string]bool, dryRun bool) (int, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory %s: %w", baseDir, err)
	}

	moved := 0
	for _, e := range entries {
		if !e.IsDir() || !nsIDs[e.Name()] {
			continue
		}
		src := filepath.Join(baseDir, e.Name())
		dst := exec.NamespaceDir(baseDir, e.Name())

		if src == dst {
			continue // already in place
		}

		if dryRun {
			moved++
			continue
		}

		if err := os.MkdirAll(filepath.Join(baseDir, exec.NsDir), 0750); err != nil {
			return 0, fmt.Errorf("failed to create ns directory in %s: %w", baseDir, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return 0, fmt.Errorf("failed to move %s to %s: %w", src, dst, err)
		}
		slog.Debug("namespace ns-relayout: relocated directory", "from", src, "to", dst)
		moved++
	}

	return moved, nil
}

// fixNsRelayoutLogPaths rewrites log file paths in status.jsonl files so they
// use the ns/ prefix: {logDir}/{nsID}/ → {logDir}/ns/{nsID}/.
func fixNsRelayoutLogPaths(dataDir, logDir string, nsIDs map[string]bool, dryRun bool) (int, error) {
	// Build replacement pairs for each namespace ID
	type replacement struct {
		oldPrefix string
		newPrefix string
	}
	var replacements []replacement
	for id := range nsIDs {
		replacements = append(replacements, replacement{
			oldPrefix: filepath.Join(logDir, id) + "/",
			newPrefix: exec.NamespaceDir(logDir, id) + "/",
		})
	}

	if len(replacements) == 0 {
		return 0, nil
	}

	fixed := 0

	err := filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Name() != "status.jsonl" {
			return nil
		}

		content, readErr := os.ReadFile(path) // #nosec G304 - path from internal walk
		if readErr != nil {
			return nil
		}

		s := string(content)
		changed := false
		for _, r := range replacements {
			if !strings.Contains(s, r.oldPrefix) {
				continue
			}
			// Protect already-correct paths, fix old paths, restore
			placeholder := "\x00NSRL\x00"
			s = strings.ReplaceAll(s, r.newPrefix, placeholder)
			s = strings.ReplaceAll(s, r.oldPrefix, r.newPrefix)
			s = strings.ReplaceAll(s, placeholder, r.newPrefix)
			changed = true
		}

		if !changed || s == string(content) {
			return nil
		}

		fixed++

		if dryRun {
			return nil
		}

		dir := filepath.Dir(path)
		tmp, tmpErr := os.CreateTemp(dir, "status-*.jsonl.tmp")
		if tmpErr != nil {
			return fmt.Errorf("failed to create temp file for %s: %w", path, tmpErr)
		}
		tmpName := tmp.Name()

		if _, wErr := tmp.WriteString(s); wErr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to write temp file for %s: %w", path, wErr)
		}
		if cErr := tmp.Close(); cErr != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to close temp file for %s: %w", path, cErr)
		}
		if rErr := os.Rename(tmpName, path); rErr != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("failed to rename temp file for %s: %w", path, rErr)
		}

		slog.Debug("namespace ns-relayout: fixed log paths in status file", "path", path)
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
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	return fileutil.FileExists(path)
}
