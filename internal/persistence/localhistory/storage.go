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
	ErrExecIDEmpty = errors.New("requestID is empty")
)

var _ models.HistoryRepository = (*historyStorage)(nil)

// historyStorage manages DAGs status files in local historyStorage with high performance and reliability.
type historyStorage struct {
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

	return &historyStorage{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// Create creates a new history record for the specified workflow ID.
// If opts.Root is not nil, it creates a new history record for a child workflow.
// If opts.Retry is true, it creates a retry record for the specified workflow ID.
func (db *historyStorage) Create(ctx context.Context, dag *digraph.DAG, timestamp time.Time, workflowID string, opts models.NewRecordOptions) (models.Record, error) {
	if workflowID == "" {
		return nil, ErrExecIDEmpty
	}

	if opts.Root != nil {
		return db.newSubRecord(ctx, dag, timestamp, workflowID, opts)
	}

	dataRoot := NewDataRoot(db.baseDir, dag.Name)
	ts := NewUTC(timestamp)

	var run *Execution
	if opts.Retry {
		r, err := dataRoot.FindByExecID(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to find execution: %w", err)
		}
		run = r
	} else {
		r, err := dataRoot.CreateRun(ts, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to create run: %w", err)
		}
		run = r
	}

	record, err := run.CreateRecord(ctx, ts, db.cache, WithDAG(dag))
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return record, nil
}

// NewSubRecord creates a new history record for a child workflow.
func (db *historyStorage) newSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, workflowID string, opts models.NewRecordOptions) (models.Record, error) {
	dataRoot := NewDataRoot(db.baseDir, opts.Root.Name)
	root, err := dataRoot.FindByExecID(ctx, opts.Root.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find root execution: %w", err)
	}

	ts := NewUTC(timestamp)

	var run *Execution
	if opts.Retry {
		r, err := root.FindChildExec(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to find child workflow record: %w", err)
		}
		run = r
	} else {
		r, err := root.CreateChildExec(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to create child workflow: %w", err)
		}
		run = r
	}

	record, err := run.CreateRecord(ctx, ts, db.cache, WithDAG(dag))
	if err != nil {
		logger.Error(ctx, "Failed to create child workflow record", "err", err)
		return nil, err
	}

	return record, nil
}

// Recent returns the most recent history records for the specified workflow name.
func (db *historyStorage) Recent(ctx context.Context, dagName string, itemLimit int) []models.Record {
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
	records := make([]models.Record, 0, len(items))
	for _, item := range items {
		record, err := item.LatestRecord(ctx, db.cache)
		if err != nil {
			logger.Error(ctx, "Failed to get latest record", "err", err)
			continue
		}
		records = append(records, record)
	}

	return records
}

// Latest returns the most recent history record for the specified workflow name.
// If latestStatusToday is true, it only returns today's status.
func (db *historyStorage) Latest(ctx context.Context, dagName string) (models.Record, error) {
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

		return exec.LatestRecord(ctx, db.cache)
	}

	// Get the latest execution data.
	latest := root.Latest(ctx, 1)
	if len(latest) == 0 {
		return nil, models.ErrNoStatusData
	}
	return latest[0].LatestRecord(ctx, db.cache)
}

// Find finds a history record by workflow ID.
func (db *historyStorage) Find(ctx context.Context, ref digraph.WorkflowRef) (models.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Find canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.WorkflowID == "" {
		return nil, ErrExecIDEmpty
	}

	root := NewDataRoot(db.baseDir, ref.Name)
	run, err := root.FindByExecID(ctx, ref.WorkflowID)

	if err != nil {
		return nil, err
	}

	return run.LatestRecord(ctx, db.cache)
}

// FindChildWorkflow finds a child workflow by its ID.
// It returns the latest record for the specified child workflow ID.
func (db *historyStorage) FindChildWorkflow(ctx context.Context, ref digraph.WorkflowRef, childWorkflowID string) (models.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindChildWorkflow canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.WorkflowID == "" {
		return nil, ErrExecIDEmpty
	}

	root := NewDataRoot(db.baseDir, ref.Name)
	run, err := root.FindByExecID(ctx, ref.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find execution: %w", err)
	}

	childWorkflow, err := run.FindChildExec(ctx, childWorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child workflow: %w", err)
	}
	return childWorkflow.LatestRecord(ctx, db.cache)
}

// RemoveOld removes old history records older than the specified retention days.
// It only removes records older than the specified retention days.
// If retentionDays is negative, no files will be removed.
// If retentionDays is zero, all files will be removed.
// If retentionDays is positive, only files older than the specified number of days will be removed.
func (db *historyStorage) RemoveOld(ctx context.Context, dagName string, retentionDays int) error {
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

// Rename renames all history records for the specified workflow name.
func (db *historyStorage) Rename(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
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
