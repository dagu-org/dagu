package jsondb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

const (
	// SubWorkflowsDir is the name of the directory where sub-workflow runs are stored.
	SubWorkflowsDir = "subworkflow_data"
)

// Run represents a single run of a DAG with its associated timestamp and request ID.
type Run struct {
	baseDir   string    // Base directory path for this run
	timestamp time.Time // Timestamp when the run was created
	requestID string    // Unique request ID for this run
}

// NewRun creates a new Run instance from a directory path.
// It parses the directory name to extract the timestamp and request ID.
func NewRun(dir string) (*Run, error) {
	run := &Run{baseDir: dir}
	matches := reRun.FindStringSubmatch(filepath.Base(dir))
	if len(matches) != 3 {
		return nil, ErrInvalidRunDir
	}
	ts, err := parseFileTimestamp(matches[1])
	if err != nil {
		return nil, err
	}
	run.timestamp = ts
	run.requestID = matches[2]
	return run, nil
}

// CreateRecord creates a new record for this run with the given timestamp.
// It creates a new attempt directory and initializes a record within it.
func (e Run) CreateRecord(_ context.Context, timestamp TimeInUTC, cache *filecache.Cache[*persistence.Status], opts ...RecordOption) (*Record, error) {
	dirName := "attempt_" + timestamp.Format(dateTimeFormatUTC)
	dir := filepath.Join(e.baseDir, dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create attempt directory: %w", err)
	}
	return NewRecord(filepath.Join(dir, "status.json"), cache, opts...), nil
}

func (e Run) CreateSubRun(_ context.Context, timestamp TimeInUTC, reqID string) (*Run, error) {
	dirName := "run_" + timestamp.Format(dateTimeFormatUTC) + "_" + reqID
	dir := filepath.Join(e.baseDir, SubWorkflowsDir, dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create sub-workflow directory: %w", err)
	}
	return NewRun(dir)
}

func (e Run) GetSubRun(_ context.Context, reqID string) (*Run, error) {
	globPattern := filepath.Join(e.baseDir, SubWorkflowsDir, "run_*_"+reqID)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list sub-workflow directories: %w", err)
	}
	// Sort the matches by timestamp
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})
	return NewRun(matches[0])
}

// LatestRecord returns the most recent record for this run.
// It searches through all attempt directories and returns the first valid record found.
func (e Run) LatestRecord(_ context.Context, cache *filecache.Cache[*persistence.Status]) (*Record, error) {
	attempts, err := listDirsSorted(e.baseDir, true, reAttempt)
	if err != nil {
		return nil, err
	}
	// Return the first valid attempt
	for _, attempt := range attempts {
		record := NewRecord(filepath.Join(e.baseDir, attempt, "status.json"), cache)
		if record.Exists() {
			return record, nil
		}
	}
	return nil, persistence.ErrNoStatusData
}

// LastUpdated returns the last modification time of the latest record.
// This is used to determine when the run was last updated.
func (e Run) LastUpdated(ctx context.Context) (time.Time, error) {
	latestRecord, err := e.LatestRecord(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	return latestRecord.ModTime()
}

// Remove deletes the entire run directory and all its contents.
func (e Run) Remove() error {
	return os.RemoveAll(e.baseDir)
}

// Regular expressions for parsing directory names
var reRun = regexp.MustCompile(`^run_(\d{8}_\d{6}_\d{3}Z)_(.*)$`)    // Matches runs directory names
var reAttempt = regexp.MustCompile(`^attempt_(\d{8}_\d{6}_\d{3}Z)$`) // Matches attempt directory names

// parseFileTimestamp converts a timestamp string from a filename into a time.Time.
// The timestamp format is expected to match dateTimeFormatUTC.
func parseFileTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(dateTimeFormatUTC, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", s, err)
	}
	return t, nil
}
