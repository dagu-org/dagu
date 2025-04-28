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
	// SubRunsDir is the name of the directory where sub-runs are stored.
	SubRunsDir = "subs"
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
	// Determine if the run is a sub-run
	parentDir := filepath.Dir(dir)
	if filepath.Base(parentDir) == SubRunsDir {
		// Sub-workflow run
		matches := reRunSub.FindStringSubmatch(filepath.Base(dir))
		if len(matches) != 2 {
			return nil, ErrInvalidRunDir
		}
		return &Run{
			baseDir:   dir,
			requestID: matches[1],
		}, nil
	}

	matches := reRun.FindStringSubmatch(filepath.Base(dir))
	if len(matches) != 3 {
		return nil, ErrInvalidRunDir
	}
	ts, err := parseRunTimestamp(matches[1])
	if err != nil {
		return nil, err
	}
	return &Run{
		baseDir:   dir,
		timestamp: ts,
		requestID: matches[2],
	}, nil
}

// CreateRecord creates a new record for this run with the given timestamp.
// It creates a new attempt directory and initializes a record within it.
func (e Run) CreateRecord(_ context.Context, ts TimeInUTC, cache *filecache.Cache[*persistence.Status], opts ...RecordOption) (*Record, error) {
	dirName := "attempt_" + formatAttemptTimestamp(ts)
	dir := filepath.Join(e.baseDir, dirName)
	// Error if the directory already exists
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("attempt directory already exists: %s", dir)
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create attempt directory: %w", err)
	}
	return NewRecord(filepath.Join(dir, "status.json"), cache, opts...), nil
}

// CreateSubRun creates a new sub-run with the given timestamp and request ID.
func (e Run) CreateSubRun(_ context.Context, reqID string) (*Run, error) {
	dirName := "sub_" + reqID
	dir := filepath.Join(e.baseDir, SubRunsDir, dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create sub-run directory: %w", err)
	}
	return NewRun(dir)
}

// FindSubRun searches for a sub-run with the specified request ID.
func (e Run) FindSubRun(_ context.Context, reqID string) (*Run, error) {
	globPattern := filepath.Join(e.baseDir, SubRunsDir, "sub_"+reqID)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list sub-run directories: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no sub-run found with request ID: %s", reqID)
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
var reRun = regexp.MustCompile(`^run_(\d{8}_\d{6}Z)_(.*)$`)          // Matches runs directory names
var reAttempt = regexp.MustCompile(`^attempt_(\d{8}_\d{6}_\d{3}Z)$`) // Matches attempt directory names
var reRunSub = regexp.MustCompile(`^sub_(.*)$`)                      // Matches sub-run directory names

// formatRunTimestamp formats a TimeInUTC instance into a string representation (without milliseconds).
// The format is "YYYYMMDD_HHMMSSZ".
// This is used for generating 'run' directory names.
func formatRunTimestamp(t TimeInUTC) string {
	return t.Time.Format(dateTimeFormatUTC)
}

// parseRunTimestamp converts a timestamp string from a filename into a time.Time.
// The timestamp format is expected to match dateTimeFormatUTC.
func parseRunTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(dateTimeFormatUTC, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", s, err)
	}
	return t, nil
}

// dateTimeFormatUTC is the format for run timestamps.
const dateTimeFormatUTC = "20060102_150405Z"

// formatAttemptTimestamp formats a TimeInUTC instance into a string representation with milliseconds.
// The format is "YYYYMMDD_HHMMSS_mmmZ" where "mmm" is the milliseconds part.
// This is used for generating 'attempt' directory names.
func formatAttemptTimestamp(t TimeInUTC) string {
	const format = "20060102_150405"
	mill := t.Time.UnixMilli()
	return t.Time.Format(format) + "_" + fmt.Sprintf("%03d", mill%1000) + "Z"
}

// dataFileExtension is the file extension for history record files.
const dataFileExtension = ".dat"
