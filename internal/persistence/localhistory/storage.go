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
	ErrReqIDEmpty = errors.New("requestID is empty")
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

// Create creates a new run record for the specified DAG run.
// If opts.Root is not nil, it creates a sub-record for the specified root DAG.
// If opts.Retry is true, it creates a retry record for the specified request ID.
func (db *historyStorage) Create(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts models.NewRecordOptions) (models.Record, error) {
	if reqID == "" {
		return nil, ErrReqIDEmpty
	}

	if opts.Root != nil {
		return db.newSubRecord(ctx, dag, timestamp, reqID, opts)
	}

	dataRoot := NewDataRoot(db.baseDir, dag.Name)
	ts := NewUTC(timestamp)

	var run *Run
	if opts.Retry {
		r, err := dataRoot.FindByReqID(ctx, reqID)
		if err != nil {
			return nil, fmt.Errorf("failed to find run: %w", err)
		}
		run = r
	} else {
		r, err := dataRoot.CreateRun(ts, reqID)
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

// NewSubRecord creates a new run record for the specified sub-run.
func (db *historyStorage) newSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts models.NewRecordOptions) (models.Record, error) {
	dataRoot := NewDataRoot(db.baseDir, opts.Root.RootName)
	rootRun, err := dataRoot.FindByReqID(ctx, opts.Root.RootID)
	if err != nil {
		return nil, fmt.Errorf("failed to find root run: %w", err)
	}

	ts := NewUTC(timestamp)

	var run *Run
	if opts.Retry {
		r, err := rootRun.FindSubRun(ctx, reqID)
		if err != nil {
			return nil, fmt.Errorf("failed to find run: %w", err)
		}
		run = r
	} else {
		r, err := rootRun.CreateSubRun(ctx, reqID)
		if err != nil {
			return nil, fmt.Errorf("failed to create sub run: %w", err)
		}
		run = r
	}

	record, err := run.CreateRecord(ctx, ts, db.cache, WithDAG(dag))
	if err != nil {
		logger.Error(ctx, "Failed to create record", "err", err)
		return nil, err
	}

	return record, nil
}

// Recent returns the most recent run records for the specified key, up to itemLimit.
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

// Latest returns the most recent run record for today.
func (db *historyStorage) Latest(ctx context.Context, dagName string) (models.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("LatestToday canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(db.baseDir, dagName)

	if db.latestStatusToday {
		startOfDay := time.Now().Truncate(24 * time.Hour)
		startOfDayInUTC := NewUTC(startOfDay)

		// Get the latest file for today
		run, err := root.LatestAfter(ctx, startOfDayInUTC)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest after: %w", err)
		}

		return run.LatestRecord(ctx, db.cache)
	}

	// Get the latest file
	latestRun := root.Latest(ctx, 1)
	if len(latestRun) == 0 {
		return nil, models.ErrNoStatusData
	}
	return latestRun[0].LatestRecord(ctx, db.cache)
}

// Find finds a run record by request ID.
func (db *historyStorage) Find(ctx context.Context, dagName, reqID string) (models.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindByReqID canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if reqID == "" {
		return nil, ErrReqIDEmpty
	}

	root := NewDataRoot(db.baseDir, dagName)
	run, err := root.FindByReqID(ctx, reqID)

	if err != nil {
		return nil, err
	}

	return run.LatestRecord(ctx, db.cache)
}

// FindSubRun finds a run record by request ID for a sub-DAG.
func (db *historyStorage) FindSubRun(ctx context.Context, name, reqID string, subRunID string) (models.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindBySubReqID canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if reqID == "" {
		return nil, ErrReqIDEmpty
	}

	root := NewDataRoot(db.baseDir, name)
	run, err := root.FindByReqID(ctx, reqID)
	if err != nil {
		return nil, err
	}

	subRun, err := run.FindSubRun(ctx, subRunID)
	if err != nil {
		return nil, err
	}
	return subRun.LatestRecord(ctx, db.cache)
}

// RemoveOld removes run records older than retentionDays for the specified key.
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

// Rename renames all run records from oldKey to newKey.
func (db *historyStorage) Rename(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(db.baseDir, oldNameOrPath)
	newRoot := NewDataRoot(db.baseDir, newNameOrPath)
	return root.Rename(ctx, newRoot)
}
