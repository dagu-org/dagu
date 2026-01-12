package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	legacymodel "github.com/dagu-org/dagu/internal/persistence/legacy/model"
)

// historyMigrator handles migration from legacy history format to new format
type historyMigrator struct {
	dagRunStore exec.DAGRunStore
	dagStore    exec.DAGStore
	dataDir     string
	dagsDir     string
}

// newHistoryMigrator creates a new history migrator
func newHistoryMigrator(
	dagRunStore exec.DAGRunStore,
	dagStore exec.DAGStore,
	dataDir string,
	dagsDir string,
) *historyMigrator {
	return &historyMigrator{
		dagRunStore: dagRunStore,
		dagStore:    dagStore,
		dataDir:     dataDir,
		dagsDir:     dagsDir,
	}
}

// migrationResult contains the result of a migration
type migrationResult struct {
	TotalDAGs    int
	TotalRuns    int
	MigratedRuns int
	SkippedRuns  int
	FailedRuns   int
	Errors       []string
}

// NeedsMigration checks if legacy data exists that needs migration
func (m *historyMigrator) NeedsMigration(_ context.Context) (bool, error) {
	dataDir := m.dataDir

	// Check if history directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return false, nil
	}

	// Check if there are any DAG directories in the history folder
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return false, fmt.Errorf("failed to read history directory: %w", err)
	}

	// Look for directories with .dat files
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dagHistoryDir := filepath.Join(dataDir, entry.Name())
		files, err := os.ReadDir(dagHistoryDir)
		if err != nil {
			continue
		}

		// Check if there are any .dat files
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".dat") {
				return true, nil
			}
		}
	}

	return false, nil
}

// Migrate performs the migration from legacy to new format
func (m *historyMigrator) Migrate(ctx context.Context) (*migrationResult, error) {
	result := &migrationResult{}

	logger.Info(ctx, "Starting history migration from legacy format")

	historyDir := m.dataDir
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return result, fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dagName := m.extractDAGName(entry.Name())
		result.TotalDAGs++

		// Enrich context with DAG name for all logs in this iteration
		dagCtx := logger.WithValues(ctx, tag.DAG(dagName))

		logger.Info(dagCtx, "Migrating DAG history")

		// Migrate all runs for this DAG
		if err := m.migrateDAGHistory(dagCtx, entry.Name(), dagName, result); err != nil {
			migrationErr := fmt.Sprintf("failed to migrate DAG %s: %s", dagName, err)
			result.Errors = append(result.Errors, migrationErr)
			logger.Error(dagCtx, "Failed to migrate DAG", tag.Error(err))
		}
	}

	return result, nil
}

// migrateDAGHistory migrates all runs for a specific DAG
func (m *historyMigrator) migrateDAGHistory(ctx context.Context, dirName, dagName string, result *migrationResult) error {
	dagHistoryDir := filepath.Join(m.dataDir, dirName)

	files, err := os.ReadDir(dagHistoryDir)
	if err != nil {
		return fmt.Errorf("failed to read DAG history directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".dat") {
			continue
		}

		result.TotalRuns++

		// Read the legacy status file to get the actual RequestID
		filePath := filepath.Join(dagHistoryDir, file.Name())
		statusFile, err := m.readLegacyStatusFile(filePath)
		if err != nil {
			result.FailedRuns++
			readErr := fmt.Sprintf("failed to read legacy status file %s: %s", file.Name(), err)
			result.Errors = append(result.Errors, readErr)
			fileCtx := logger.WithValues(ctx,
				tag.File(file.Name()),
				tag.Path(filePath),
			)
			logger.Error(fileCtx, "Failed to read legacy status file", tag.Error(err))
			continue
		}

		if statusFile == nil || statusFile.Status.RequestID == "" || statusFile.Status.Status == core.NotStarted {
			result.SkippedRuns++
			err := fmt.Sprintf("skipped invalid status file %s, RequestID=%s, Status=%s", file.Name(), statusFile.Status.RequestID, statusFile.Status.Status.String())
			result.Errors = append(result.Errors, err)
			fileCtx := logger.WithValues(ctx, tag.File(file.Name()))
			logger.Debug(fileCtx, "Skipping file with no valid status")
			continue
		}

		requestID := statusFile.Status.RequestID

		// Enrich context with run identifiers for all logs in this iteration
		runCtx := logger.WithValues(ctx, tag.RequestID(requestID))

		// Check if already migrated
		if m.isAlreadyMigrated(ctx, statusFile.Status.Name, requestID) {
			result.SkippedRuns++
			logger.Debug(runCtx, "Run already migrated")
			continue
		}

		// Convert and save - pass the directory name (without hash) as additional hint
		if err := m.migrateRun(ctx, statusFile, dagName); err != nil {
			result.FailedRuns++
			migrationErr := fmt.Sprintf("failed to migrate run %s: %s", requestID, err)
			result.Errors = append(result.Errors, migrationErr)
			logger.Error(runCtx, "Failed to migrate run", tag.Error(err))
			continue
		}

		result.MigratedRuns++
	}

	return nil
}

// migrateRun converts and saves a single run
func (m *historyMigrator) migrateRun(ctx context.Context, legacyStatusFile *legacymodel.StatusFile, dirBasedDagName string) error {
	legacyStatus := &legacyStatusFile.Status

	// Load the DAG definition - try both the status name and directory-based name
	dag, err := m.loadDAGForMigration(ctx, legacyStatus.Name, dirBasedDagName)
	if err != nil {
		return fmt.Errorf("failed to load DAG %s: %w", legacyStatus.Name, err)
	}

	// Convert legacy status to new format
	newStatus := m.convertStatus(legacyStatus, dag)

	// Parse started time to get timestamp for CreateAttempt
	startedAt, _ := m.parseTime(legacyStatus.StartedAt)
	if startedAt.IsZero() {
		return fmt.Errorf("invalid history data: no started at time: %s - %s", legacyStatus.Name, legacyStatus.RequestID)
	}

	// Create attempt in new store
	attempt, err := m.dagRunStore.CreateAttempt(ctx, dag, startedAt, newStatus.DAGRunID, exec.NewDAGRunAttemptOptions{
		RootDAGRun: nil, // No hierarchy info in legacy format
		Retry:      false,
	})
	if err != nil {
		return fmt.Errorf("failed to create attempt: %w", err)
	}

	// Open the attempt for writing
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open attempt: %w", err)
	}

	// Write the converted status
	if err := attempt.Write(ctx, *newStatus); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	// Close the attempt
	if err := attempt.Close(ctx); err != nil {
		return fmt.Errorf("failed to close attempt: %w", err)
	}

	logger.Debug(ctx, "Migrated run",
		tag.DAG(legacyStatus.Name),
		tag.RequestID(legacyStatus.RequestID),
	)

	return nil
}

// convertStatus converts legacy status to new DAGRunStatus format
func (m *historyMigrator) convertStatus(legacy *legacymodel.Status, dag *core.DAG) *exec.DAGRunStatus {
	// Convert timestamps
	startedAt, _ := m.parseTime(legacy.StartedAt)
	finishedAt, _ := m.parseTime(legacy.FinishedAt)

	// Create createdAt timestamp based on startedAt
	createdAt := time.Now().UnixMilli()
	if !startedAt.IsZero() {
		createdAt = startedAt.UnixMilli()
	}

	status := &exec.DAGRunStatus{
		Name:       legacy.Name,
		DAGRunID:   legacy.RequestID,
		Status:     legacy.Status,
		PID:        exec.PID(legacy.PID),
		Log:        legacy.Log,
		Nodes:      make([]*exec.Node, 0),
		Params:     legacy.Params,
		ParamsList: legacy.ParamsList,
		CreatedAt:  createdAt,
		StartedAt:  formatTime(startedAt),
		FinishedAt: formatTime(finishedAt),
		QueuedAt:   formatTime(startedAt), // Use StartedAt as QueuedAt for migration
	}

	// Convert nodes
	for _, node := range legacy.Nodes {
		status.Nodes = append(status.Nodes, m.convertNode(node))
	}

	// Convert handler nodes
	if legacy.OnExit != nil {
		status.OnExit = m.convertNode(legacy.OnExit)
	}
	if legacy.OnSuccess != nil {
		status.OnSuccess = m.convertNode(legacy.OnSuccess)
	}
	if legacy.OnFailure != nil {
		status.OnFailure = m.convertNode(legacy.OnFailure)
	}
	if legacy.OnCancel != nil {
		status.OnCancel = m.convertNode(legacy.OnCancel)
	}

	// Set preconditions from DAG if available
	if dag != nil {
		status.Preconditions = dag.Preconditions
	}

	return status
}

// convertNode converts legacy node to new Node format
func (m *historyMigrator) convertNode(legacy *legacymodel.Node) *exec.Node {
	node := &exec.Node{
		Step:       legacy.Step,
		Status:     legacy.Status,
		Error:      legacy.Error,
		RetryCount: legacy.RetryCount,
		DoneCount:  legacy.DoneCount,
		StartedAt:  legacy.StartedAt,
		FinishedAt: legacy.FinishedAt,
		RetriedAt:  legacy.RetriedAt,
		Stdout:     legacy.Log,
	}

	return node
}

// parseTime attempts to parse various time formats
func (m *historyMigrator) parseTime(timeStr string) (time.Time, error) {
	if timeStr == "" || timeStr == "-" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try various formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// formatTime formats a time value for the new format
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// extractDAGName extracts the DAG name from directory name
func (m *historyMigrator) extractDAGName(dirName string) string {
	// Directory format: {dag-name}-{hash}
	// Just remove the last part after hyphen if it looks like a hash
	lastHyphen := strings.LastIndex(dirName, "-")
	if lastHyphen == -1 {
		return dirName
	}

	// Check if the part after hyphen is all hex chars
	suffix := dirName[lastHyphen+1:]
	for _, ch := range suffix {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) { //nolint:staticcheck
			return dirName // Not a hash, return full name
		}
	}

	return dirName[:lastHyphen]
}

// readLegacyStatusFile reads a legacy status file directly
func (m *historyMigrator) readLegacyStatusFile(filePath string) (*legacymodel.StatusFile, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// The legacy status files contain multiple JSON lines, read the last one
	lines := strings.Split(string(data), "\n")
	var lastValidLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			lastValidLine = line
			break
		}
	}

	if lastValidLine == "" {
		return nil, fmt.Errorf("no valid status data found in file")
	}

	var statusFile legacymodel.StatusFile
	if err := json.Unmarshal([]byte(lastValidLine), &statusFile.Status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return &statusFile, nil
}

// isAlreadyMigrated checks if a run has already been migrated
func (m *historyMigrator) isAlreadyMigrated(ctx context.Context, dagName, requestID string) bool {
	attempt, err := m.dagRunStore.FindAttempt(ctx, exec.NewDAGRunRef(dagName, requestID))
	if err != nil || attempt == nil {
		return false
	}

	status, err := attempt.ReadStatus(ctx)
	return err == nil && status != nil
}

// loadDAGForMigration attempts to load the DAG definition
func (m *historyMigrator) loadDAGForMigration(ctx context.Context, statusDagName, dirBasedDagName string) (*core.DAG, error) {
	// Try both DAG names as candidates
	candidates := []string{statusDagName}
	if dirBasedDagName != "" && dirBasedDagName != statusDagName {
		candidates = append(candidates, dirBasedDagName)
	}

	// Try to find the DAG file with different extensions
	extensions := []string{".yaml", ".yml"}

	for _, candidate := range candidates {
		for _, ext := range extensions {
			path := filepath.Join(m.dagsDir, candidate+ext)
			if _, err := os.Stat(path); err == nil {
				dag, err := m.dagStore.GetDetails(ctx, path)
				if err == nil && dag != nil {
					return dag, nil
				}
			}
		}
	}

	// If we can't find the DAG, create a minimal one
	return &core.DAG{
		Name: statusDagName,
	}, nil
}

// MoveLegacyData moves individual legacy DAG directories to an archive location after successful migration
func (m *historyMigrator) MoveLegacyData(ctx context.Context) error {
	archiveDir := filepath.Join(m.dataDir, fmt.Sprintf("history_migrated_%s", time.Now().Format("20060102_150405")))

	// Create archive directory
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	logger.Info(ctx, "Moving legacy history directories to archive",
		tag.ArchiveDir(archiveDir),
	)

	// Read data directory entries
	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	movedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(m.dataDir, entry.Name())

		// Check if this directory contains .dat files (legacy history)
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		hasDataFiles := false
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".dat") {
				hasDataFiles = true
				break
			}
		}

		if !hasDataFiles {
			continue
		}

		// Move this legacy directory to archive
		archivePath := filepath.Join(archiveDir, entry.Name())
		if err := os.Rename(dirPath, archivePath); err != nil {
			logger.Warn(ctx, "Failed to move legacy directory", tag.Dir(entry.Name()), tag.Error(err))
		} else {
			movedCount++
			logger.Debug(ctx, "Moved legacy directory", tag.Dir(entry.Name()))
		}
	}

	logger.Info(ctx, "Legacy history data archived successfully",
		slog.String("location", archiveDir),
		tag.DirsProcessed(movedCount),
	)
	return nil
}
