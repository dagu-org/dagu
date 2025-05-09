package localhistory

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Error definitions for common issues
var (
	ErrWorkflowIDEmpty = errors.New("workflow ID is empty")
)

var _ models.HistoryRepository = (*localStorage)(nil)

// localStorage manages DAGs status files in local localStorage with high performance and reliability.
type localStorage struct {
	baseDir           string                          // Base directory for all status files
	latestStatusToday bool                            // Whether to only return today's status
	cache             *fileutil.Cache[*models.Status] // Optional cache for read operations
	maxWorkers        int                             // Maximum number of parallel workers
}

// HistoryStorageOption defines functional options for configuring local.
type HistoryStorageOption func(*HistoryStorageOptions)

// HistoryStorageOptions holds configuration options for local.
type HistoryStorageOptions struct {
	FileCache         *fileutil.Cache[*models.Status] // Optional cache for status files
	LatestStatusToday bool                            // Whether to only return today's status
	MaxWorkers        int                             // Maximum number of parallel workers
	OperationTimeout  time.Duration                   // Timeout for operations
}

// WithHistoryFileCache sets the file cache for local.
func WithHistoryFileCache(cache *fileutil.Cache[*models.Status]) HistoryStorageOption {
	return func(o *HistoryStorageOptions) {
		o.FileCache = cache
	}
}

// WithLatestStatusToday sets whether to only return today's status.
func WithLatestStatusToday(latestStatusToday bool) HistoryStorageOption {
	return func(o *HistoryStorageOptions) {
		o.LatestStatusToday = latestStatusToday
	}
}

// New creates a new JSONDB instance with the specified options.
func New(baseDir string, opts ...HistoryStorageOption) models.HistoryRepository {
	options := &HistoryStorageOptions{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &localStorage{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// CreateRun creates a new history record for the specified workflow ID.
// If opts.Root is not nil, it creates a new history record for a child workflow.
// If opts.Retry is true, it creates a retry record for the specified workflow ID.
func (db *localStorage) CreateRun(ctx context.Context, dag *digraph.DAG, timestamp time.Time, workflowID string, opts models.NewRunOptions) (models.Run, error) {
	if workflowID == "" {
		return nil, ErrWorkflowIDEmpty
	}

	if opts.Root != nil {
		return db.newChildRecord(ctx, dag, timestamp, workflowID, opts)
	}

	dataRoot := NewDataRoot(db.baseDir, dag.Name)
	ts := NewUTC(timestamp)

	var run *Workflow
	if opts.Retry {
		r, err := dataRoot.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to find execution: %w", err)
		}
		run = r
	} else {
		r, err := dataRoot.CreateWorkflow(ts, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to create run: %w", err)
		}
		run = r
	}

	record, err := run.CreateRun(ctx, ts, db.cache, WithDAG(dag))
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return record, nil
}

// newChildRecord creates a new history record for a child workflow.
func (db *localStorage) newChildRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, workflowID string, opts models.NewRunOptions) (models.Run, error) {
	dataRoot := NewDataRoot(db.baseDir, opts.Root.Name)
	root, err := dataRoot.FindByWorkflowID(ctx, opts.Root.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find root execution: %w", err)
	}

	ts := NewUTC(timestamp)

	var run *Workflow
	if opts.Retry {
		r, err := root.FindChildWorkflow(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to find child workflow record: %w", err)
		}
		run = r
	} else {
		r, err := root.CreateChildWorkflow(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to create child workflow: %w", err)
		}
		run = r
	}

	record, err := run.CreateRun(ctx, ts, db.cache, WithDAG(dag))
	if err != nil {
		logger.Error(ctx, "Failed to create child workflow record", "err", err)
		return nil, err
	}

	return record, nil
}

// RecentRuns returns the most recent history records for the specified workflow name.
func (db *localStorage) RecentRuns(ctx context.Context, dagName string, itemLimit int) []models.Run {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		logger.Errorf(ctx, "Recent canceled: %v", ctx.Err())
		return nil
	default:
		// Continue with operation
	}

	if itemLimit <= 0 {
		logger.Warnf(ctx, "Invalid itemLimit %d, using default of 10", itemLimit)
		itemLimit = 10
	}

	// Get the latest matches
	root := NewDataRoot(db.baseDir, dagName)
	items := root.Latest(ctx, itemLimit)

	// Get the latest record for each item
	records := make([]models.Run, 0, len(items))
	for _, item := range items {
		record, err := item.LatestRun(ctx, db.cache)
		if err != nil {
			logger.Error(ctx, "Failed to get latest record", "err", err)
			continue
		}
		records = append(records, record)
	}

	return records
}

// LatestRun returns the most recent history record for the specified workflow name.
// If latestStatusToday is true, it only returns today's status.
func (db *localStorage) LatestRun(ctx context.Context, dagName string) (models.Run, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Latest canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(db.baseDir, dagName)

	if db.latestStatusToday {
		startOfDay := time.Now().Truncate(24 * time.Hour)
		startOfDayInUTC := NewUTC(startOfDay)

		// Get the latest execution data after the start of the day.
		exec, err := root.LatestAfter(ctx, startOfDayInUTC)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest after: %w", err)
		}

		return exec.LatestRun(ctx, db.cache)
	}

	// Get the latest execution data.
	latest := root.Latest(ctx, 1)
	if len(latest) == 0 {
		return nil, models.ErrNoStatusData
	}
	return latest[0].LatestRun(ctx, db.cache)
}

// FindRun finds a history record by workflow ID.
func (db *localStorage) FindRun(ctx context.Context, ref digraph.WorkflowRef) (models.Run, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Find canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.WorkflowID == "" {
		return nil, ErrWorkflowIDEmpty
	}

	root := NewDataRoot(db.baseDir, ref.Name)
	run, err := root.FindByWorkflowID(ctx, ref.WorkflowID)

	if err != nil {
		return nil, err
	}

	return run.LatestRun(ctx, db.cache)
}

// FindChildWorkflowRun finds a child workflow by its ID.
// It returns the latest record for the specified child workflow ID.
func (db *localStorage) FindChildWorkflowRun(ctx context.Context, ref digraph.WorkflowRef, childWorkflowID string) (models.Run, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindChildWorkflow canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.WorkflowID == "" {
		return nil, ErrWorkflowIDEmpty
	}

	root := NewDataRoot(db.baseDir, ref.Name)
	run, err := root.FindByWorkflowID(ctx, ref.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find execution: %w", err)
	}

	childWorkflow, err := run.FindChildWorkflow(ctx, childWorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child workflow: %w", err)
	}
	return childWorkflow.LatestRun(ctx, db.cache)
}

// RemoveOldWorkflows removes old history records older than the specified retention days.
// It only removes records older than the specified retention days.
// If retentionDays is negative, no files will be removed.
// If retentionDays is zero, all files will be removed.
// If retentionDays is positive, only files older than the specified number of days will be removed.
func (db *localStorage) RemoveOldWorkflows(ctx context.Context, dagName string, retentionDays int) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("RemoveOld canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if retentionDays < 0 {
		logger.Warnf(ctx, "Negative retentionDays %d, no files will be removed", retentionDays)
		return nil
	}

	root := NewDataRoot(db.baseDir, dagName)
	return root.RemoveOld(ctx, retentionDays)
}

// RenameWorkflows renames all history records for the specified workflow name.
func (db *localStorage) RenameWorkflows(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("Rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(db.baseDir, oldNameOrPath)
	newRoot := NewDataRoot(db.baseDir, newNameOrPath)
	return root.Rename(ctx, newRoot)
}
