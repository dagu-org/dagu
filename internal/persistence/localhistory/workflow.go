package localhistory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"
)

// Error definitions for directory structure validation
var (
	ErrInvalidWorkflowsDir = errors.New("invalid workflows directory")
)

const (
	// ChildWorkflowsDir is the name of the directory where status files for child workflow data are stored.
	ChildWorkflowsDir = "children"

	// ChildWorkflowDirPrefix is the prefix for child workflow directories.
	ChildWorkflowDirPrefix = "child_"

	// WorkflowDirPrefix is the prefix for workflow directories.
	WorkflowDirPrefix = "workflow_"

	// RunDirPrefix is the prefix for run directories.
	RunDirPrefix = "run_"
)

// JSONLStatusFile is the name of the status file for each workflow run.
// It contains the status of the workflow in JSON Lines format.
// While running the DAG, new lines are appended to this file on each status update.
// After finishing the run, this file will be compacted into a single JSON line file.
const JSONLStatusFile = "status.jsonl"

// Workflow represents a single run of a DAG with its associated timestamp and workflow ID.
type Workflow struct {
	baseDir    string    // Base directory path for this run
	timestamp  time.Time // Timestamp when the run was created
	workflowID string    // Unique workflow ID for this run
}

// NewWorkflow creates a new Run instance from a directory path.
// It parses the directory name to extract the timestamp and workflow ID.
func NewWorkflow(dir string) (*Workflow, error) {
	// Determine if the run is a child workflow
	parentDir := filepath.Dir(dir)
	if filepath.Base(parentDir) == ChildWorkflowsDir {
		matches := reChildWorkflow.FindStringSubmatch(filepath.Base(dir))
		if len(matches) != 2 {
			return nil, ErrInvalidWorkflowsDir
		}
		return &Workflow{
			baseDir:    dir,
			workflowID: matches[1],
		}, nil
	}

	matches := reWorkflow.FindStringSubmatch(filepath.Base(dir))
	if len(matches) != 3 {
		return nil, ErrInvalidWorkflowsDir
	}
	ts, err := parseRunTimestamp(matches[1])
	if err != nil {
		return nil, err
	}
	return &Workflow{
		baseDir:    dir,
		timestamp:  ts,
		workflowID: matches[2],
	}, nil
}

// CreateRun creates a new run for the workflow with the given timestamp.
// It creates a new run directory and initializes a record within it.
func (e Workflow) CreateRun(_ context.Context, ts TimeInUTC, cache *fileutil.Cache[*models.Status], opts ...RunOption) (*Run, error) {
	dir := filepath.Join(e.baseDir, RunDirPrefix+formatRunTimestamp(ts))
	// Error if the directory already exists
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("run directory already exists: %s", dir)
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create the run directory: %w", err)
	}
	return NewRun(filepath.Join(dir, JSONLStatusFile), cache, opts...), nil
}

// CreateChildWorkflow creates a new child workflow with the given timestamp and workflow ID.
func (e Workflow) CreateChildWorkflow(_ context.Context, workflowID string) (*Workflow, error) {
	dirName := "child_" + workflowID
	dir := filepath.Join(e.baseDir, ChildWorkflowsDir, dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create child workflow directory: %w", err)
	}
	return NewWorkflow(dir)
}

// FindChildWorkflow searches for a child workflow with the specified workflow ID.
func (e Workflow) FindChildWorkflow(_ context.Context, workflowID string) (*Workflow, error) {
	globPattern := filepath.Join(e.baseDir, ChildWorkflowsDir, "child_"+workflowID)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list child workflow directories: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching child workflow found for ID %s (glob=%s): %w", workflowID, globPattern, models.ErrWorkflowIDNotFound)
	}
	// Sort the matches by timestamp
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})
	return NewWorkflow(matches[0])
}

// LatestRun returns the most recent run for the workflow.
// It searches through all run directories and returns the first valid runs found.
func (e Workflow) LatestRun(_ context.Context, cache *fileutil.Cache[*models.Status]) (*Run, error) {
	runDirs, err := listDirsSorted(e.baseDir, true, reRun)
	if err != nil {
		return nil, fmt.Errorf("failed to list run directories: %w", err)
	}
	// Return the first valid run
	for _, runDir := range runDirs {
		run := NewRun(filepath.Join(e.baseDir, runDir, JSONLStatusFile), cache)
		if run.Exists() {
			return run, nil
		}
	}
	return nil, models.ErrNoStatusData
}

// LastUpdated returns the last modification time of the latest record.
// This is used to determine when the run was last updated.
func (e Workflow) LastUpdated(ctx context.Context) (time.Time, error) {
	run, err := e.LatestRun(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	return run.ModTime()
}

// Remove deletes the entire run directory and all its contents.
func (e Workflow) Remove() error {
	return os.RemoveAll(e.baseDir)
}

// Regular expressions for parsing directory names
var reWorkflow = regexp.MustCompile(`^` + WorkflowDirPrefix + `(\d{8}_\d{6}Z)_(.*)$`) // Matches workflow directory names
var reRun = regexp.MustCompile(`^` + RunDirPrefix + `(\d{8}_\d{6}_\d{3}Z)$`)          // Matches run directory names
var reChildWorkflow = regexp.MustCompile(`^` + ChildWorkflowDirPrefix + `(.*)$`)      // Matches child workflow directory names

// formatWorkflowTimestamp formats a TimeInUTC instance into a string representation (without milliseconds).
// The format is "YYYYMMDD_HHMMSSZ".
// This is used for generating 'run' directory names.
func formatWorkflowTimestamp(t TimeInUTC) string {
	return t.Format(dateTimeFormatUTC)
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

// formatRunTimestamp formats a TimeInUTC instance into a string representation with milliseconds.
// The format is "YYYYMMDD_HHMMSS_mmmZ" where "mmm" is the milliseconds part.
// This is used for generating 'run' directory names.
func formatRunTimestamp(t TimeInUTC) string {
	const format = "20060102_150405"
	mill := t.UnixMilli()
	return t.Format(format) + "_" + fmt.Sprintf("%03d", mill%1000) + "Z"
}
