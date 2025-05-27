package localhistory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Error definitions for directory structure validation
var (
	ErrInvalidDAGRunsDir = errors.New("invalid DAG-runs directory name")
)

const (
	// ChildDAGRunsDir is the name of the directory where status files for child DAG-runs are stored.
	ChildDAGRunsDir = "children"

	// ChildDAGRunDirPrefix is the prefix for child DAG-run directories.
	ChildDAGRunDirPrefix = "child_"

	// DAGRunDirPrefix is the prefix for DAG-run directories.
	DAGRunDirPrefix = "dag-run_"

	// AttemptDirPrefix is the prefix for attempt directories.
	AttemptDirPrefix = "attempt_"
)

// JSONLStatusFile is the name of the status file for each DAG run.
// It contains the status of the DAG-run in JSON Lines format.
// While running the DAG-run, new lines are appended to this file on each status update.
// After finishing the run, this file will be compacted into a single JSON line file.
const JSONLStatusFile = "status.jsonl"

// DAGRun represents a DAG-run with its associated timestamp and run ID.
type DAGRun struct {
	baseDir   string    // Base directory path for this run
	timestamp time.Time // Timestamp when the run was created
	dagRunID  string    // Unique identifier for the DAG run
}

// NewDAGRun creates a new Run instance from a directory path.
// It parses the directory name to extract the timestamp and DAG-run ID.
func NewDAGRun(dir string) (*DAGRun, error) {
	// Determine if the run is a child DAG-run or a regular DAG-run.
	parentDir := filepath.Dir(dir)
	if filepath.Base(parentDir) == ChildDAGRunsDir {
		matches := reChildDAGRunDir.FindStringSubmatch(filepath.Base(dir))
		if len(matches) != 2 {
			return nil, ErrInvalidDAGRunsDir
		}
		return &DAGRun{
			baseDir:  dir,
			dagRunID: matches[1],
		}, nil
	}

	matches := reDAGRunDir.FindStringSubmatch(filepath.Base(dir))
	if len(matches) != 3 {
		return nil, ErrInvalidDAGRunsDir
	}
	ts, err := parseDAGRunTimestamp(matches[1])
	if err != nil {
		return nil, err
	}
	return &DAGRun{
		baseDir:   dir,
		timestamp: ts,
		dagRunID:  matches[2],
	}, nil
}

// CreateAttempt creates a new Attempt for the DAG-run with the given timestamp.
// It creates a new Attempt directory and initializes a record within it.
func (dr DAGRun) CreateAttempt(_ context.Context, ts models.TimeInUTC, cache *fileutil.Cache[*models.DAGRunStatus], opts ...AttemptOption) (*Attempt, error) {
	attID, err := genAttemptID()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(dr.baseDir, AttemptDirPrefix+formatAttemptTimestamp(ts)+"_"+attID)
	// Error if the directory already exists
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("run directory already exists: %s", dir)
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create the run directory: %w", err)
	}
	return NewAttempt(filepath.Join(dir, JSONLStatusFile), cache, opts...)
}

// CreateChildDAGRun creates a new child DAG-run with the given timestamp and DAG-run ID.
func (dr DAGRun) CreateChildDAGRun(_ context.Context, dagRunID string) (*DAGRun, error) {
	dirName := "child_" + dagRunID
	dir := filepath.Join(dr.baseDir, ChildDAGRunsDir, dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create child DAG-run directory: %w", err)
	}
	return NewDAGRun(dir)
}

// FindChildDAGRun searches for a child DAG-run by its run ID.
func (dr DAGRun) FindChildDAGRun(_ context.Context, dagRunID string) (*DAGRun, error) {
	globPattern := filepath.Join(dr.baseDir, ChildDAGRunsDir, "child_"+dagRunID)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list child DAG-runs: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching child DAG-run found for ID %s (glob=%s): %w", dagRunID, globPattern, models.ErrDAGRunIDNotFound)
	}
	// Sort the matches by timestamp
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})
	return NewDAGRun(matches[0])
}

func (dr DAGRun) ListChildDAGRuns(ctx context.Context) ([]*DAGRun, error) {
	childDir := filepath.Join(dr.baseDir, ChildDAGRunsDir)
	entries, err := os.ReadDir(childDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*DAGRun{}, nil
		}
		return nil, fmt.Errorf("failed to read child DAG-runs directory: %w", err)
	}

	var dagRuns []*DAGRun
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// check if the directory name matches the child DAG-run directory pattern
		if !reChildDAGRunDir.MatchString(entry.Name()) {
			continue
		}

		childDAGRun, err := NewDAGRun(filepath.Join(childDir, entry.Name()))
		if err != nil {
			logger.Error(ctx, "failed to read child DAG-run data", "err", err, "dagRunId", dr.dagRunID, "childDAGRunDir", entry.Name())
			continue
		}
		dagRuns = append(dagRuns, childDAGRun)
	}
	return dagRuns, nil
}

// ListAttempts returns a list of all attempts for the DAG-run.
func (dr DAGRun) ListAttempts(ctx context.Context) ([]*Attempt, error) {
	attDirs, err := listDirsSorted(dr.baseDir, true, reAttemptDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list run directories: %w", err)
	}
	var attempts []*Attempt
	for _, attDir := range attDirs {
		run, err := NewAttempt(filepath.Join(dr.baseDir, attDir, JSONLStatusFile), nil)
		if err != nil {
			logger.Error(ctx, "failed to read a run data: %w", err)
			continue
		}
		if run.Exists() {
			attempts = append(attempts, run)
		}
	}
	return attempts, nil
}

// LatestAttempt returns the most recent Attempt for the DAG-run.
// It searches through all run directories and returns the first valid Attempt found.
func (dr DAGRun) LatestAttempt(ctx context.Context, cache *fileutil.Cache[*models.DAGRunStatus]) (*Attempt, error) {
	attDirs, err := listDirsSorted(dr.baseDir, true, reAttemptDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list run directories: %w", err)
	}
	// Return the first valid run
	for _, attDir := range attDirs {
		att, err := NewAttempt(filepath.Join(dr.baseDir, attDir, JSONLStatusFile), cache)
		if err != nil {
			logger.Error(ctx, "failed to read a run data: %w", err)
			continue
		}
		if att.Exists() {
			return att, nil
		}
	}
	return nil, models.ErrNoStatusData
}

// Remove deletes the entire DAG-run directory and all its contents.
func (dr DAGRun) Remove(ctx context.Context) error {
	if err := dr.removeLogFiles(ctx); err != nil {
		logger.Error(ctx, "failed to remove log files", "err", err, "dagRunId", dr.dagRunID)
	}
	return os.RemoveAll(dr.baseDir)
}

func (dr DAGRun) removeLogFiles(ctx context.Context) error {
	// Remove the log files
	deleteFiles, err := dr.listLogFiles(ctx)
	if err != nil {
		logger.Error(ctx, "failed to list log files to remove", "err", err, "dagRunId", dr.dagRunID)
	}

	children, err := dr.ListChildDAGRuns(ctx)
	if err != nil {
		logger.Error(ctx, "failed to list child DAG-runs", "err", err, "dagRunId", dr.dagRunID)
	}
	for _, child := range children {
		childLogFiles, err := child.listLogFiles(ctx)
		if err != nil {
			logger.Error(ctx, "failed to list log files for child DAG-run", "err", err, "dagRunId", child.dagRunID)
		}
		deleteFiles = append(deleteFiles, childLogFiles...)
	}

	parentDirs := make(map[string]struct{})

	// Remove all log files.
	for _, file := range deleteFiles {
		if err := os.Remove(file); err != nil {
			logger.Error(ctx, "failed to remove log file", "err", err, "dagRunId", dr.dagRunID, "file", file)
		}
		parentDirs[filepath.Dir(file)] = struct{}{}
	}

	// Remove parent dirs if they are empty.
	for p := range parentDirs {
		_ = os.Remove(p)
	}

	return nil
}

// listLogFiles lists all log files associated with the DAG-run.
func (dr DAGRun) listLogFiles(ctx context.Context) ([]string, error) {
	// List all log files in the DAG-run directory and its attempts.
	runs, err := dr.ListAttempts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	var logFiles []string
	for _, run := range runs {
		status, err := run.ReadStatus(ctx)
		if err != nil {
			logger.Error(ctx, "failed to read status for run", "err", err, "dagRunId", dr.dagRunID, "run", run.ID())
			continue
		}
		logFiles = append(logFiles, status.Log)
		for _, n := range status.Nodes {
			logFiles = append(logFiles, n.Stdout, n.Stderr)
		}
		for _, n := range []*models.Node{
			status.OnSuccess, status.OnExit, status.OnFailure, status.OnCancel,
		} {
			if n == nil {
				continue
			}
			logFiles = append(logFiles, n.Stdout, n.Stderr)
		}
	}

	return logFiles, nil
}

// Regular expressions for parsing directory names
var reDAGRunDir = regexp.MustCompile(`^` + DAGRunDirPrefix + `(\d{8}_\d{6}Z)_(.*)$`)         // Matches DAG-run directory names
var reAttemptDir = regexp.MustCompile(`^` + AttemptDirPrefix + `(\d{8}_\d{6}_\d{3}Z)_(.*)$`) // Matches attempt directory names
var reChildDAGRunDir = regexp.MustCompile(`^` + ChildDAGRunDirPrefix + `(.*)$`)              // Matches child DAG-run directory names

// formatDAGRunTimestamp formats a models.TimeInUTC instance into a string representation (without milliseconds).
// The format is "YYYYMMDD_HHMMSSZ".
// This is used for generating 'run' directory names.
func formatDAGRunTimestamp(t models.TimeInUTC) string {
	return t.Format(dateTimeFormatUTC)
}

// parseDAGRunTimestamp converts a timestamp string from a filename into a time.Time.
// The timestamp format is expected to match dateTimeFormatUTC.
func parseDAGRunTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(dateTimeFormatUTC, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", s, err)
	}
	return t, nil
}

// dateTimeFormatUTC is the format for run timestamps.
const dateTimeFormatUTC = "20060102_150405Z"

// formatAttemptTimestamp formats a models.TimeInUTC instance into a string representation with milliseconds.
// The format is "YYYYMMDD_HHMMSS_mmmZ" where "mmm" is the milliseconds part.
func formatAttemptTimestamp(t models.TimeInUTC) string {
	const format = "20060102_150405"
	mill := t.UnixMilli()
	return t.Format(format) + "_" + fmt.Sprintf("%03d", mill%1000) + "Z"
}

// genAttemptID generates unique run ID
func genAttemptID() (string, error) {
	// 3 bytes → 6 hex characters
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
