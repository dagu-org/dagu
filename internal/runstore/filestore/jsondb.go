package filestore

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/runstore"
)

// Error definitions for common issues
var (
	ErrInvalidPath        = errors.New("invalid path")
	ErrRequestIDEmpty     = errors.New("requestID is empty")
	ErrRootRequestIDEmpty = errors.New("root requestID is empty")
)

var _ runstore.Store = (*fileStore)(nil)

// fileStore manages DAGs status files in local storage with high performance and reliability.
type fileStore struct {
	baseDir           string                            // Base directory for all status files
	latestStatusToday bool                              // Whether to only return today's status
	cache             *fileutil.Cache[*runstore.Status] // Optional cache for read operations
	maxWorkers        int                               // Maximum number of parallel workers
}

// Option defines functional options for configuring local.
type Option func(*Options)

// Options holds configuration options for local.
type Options struct {
	FileCache         *fileutil.Cache[*runstore.Status] // Optional cache for status files
	LatestStatusToday bool                              // Whether to only return today's status
	MaxWorkers        int                               // Maximum number of parallel workers
	OperationTimeout  time.Duration                     // Timeout for operations
}

// WithFileCache sets the file cache for local.
func WithFileCache(cache *fileutil.Cache[*runstore.Status]) Option {
	return func(o *Options) {
		o.FileCache = cache
	}
}

// WithLatestStatusToday sets whether to only return today's status.
func WithLatestStatusToday(latestStatusToday bool) Option {
	return func(o *Options) {
		o.LatestStatusToday = latestStatusToday
	}
}

// New creates a new JSONDB instance with the specified options.
func New(baseDir string, opts ...Option) *fileStore {
	options := &Options{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &fileStore{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// NewRecord creates a new runstore record for the specified DAG run.
// If opts.Root is not nil, it creates a sub-record for the specified root DAG.
// If opts.Retry is true, it creates a retry record for the specified request ID.
func (db *fileStore) NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts runstore.NewRecordOptions) (runstore.Record, error) {
	if reqID == "" {
		return nil, ErrRequestIDEmpty
	}

	if opts.Root != nil {
		return db.newSubRecord(ctx, dag, timestamp, reqID, opts)
	}

	dataRoot := NewDataRoot(db.baseDir, dag.Name)
	ts := NewUTC(timestamp)

	var run *Run
	if opts.Retry {
		r, err := dataRoot.FindByRequestID(ctx, reqID)
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

// NewSubRecord creates a new runstore record for the specified sub-run.
func (db *fileStore) newSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts runstore.NewRecordOptions) (runstore.Record, error) {
	dataRoot := NewDataRoot(db.baseDir, opts.Root.Name)
	rootRun, err := dataRoot.FindByRequestID(ctx, opts.Root.RequestID)
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

// Recent returns the most recent runstore records for the specified key, up to itemLimit.
func (db *fileStore) Recent(ctx context.Context, dagName string, itemLimit int) []runstore.Record {
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
	records := make([]runstore.Record, 0, len(items))
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

// Latest returns the most recent runstore record for today.
func (db *fileStore) Latest(ctx context.Context, dagName string) (runstore.Record, error) {
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
		return nil, runstore.ErrNoStatusData
	}
	return latestRun[0].LatestRecord(ctx, db.cache)
}

// FindByRequestID finds a runstore record by request ID.
func (db *fileStore) FindByRequestID(ctx context.Context, dagName, reqID string) (runstore.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindByRequestID canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if reqID == "" {
		return nil, ErrRequestIDEmpty
	}

	root := NewDataRoot(db.baseDir, dagName)
	run, err := root.FindByRequestID(ctx, reqID)

	if err != nil {
		return nil, err
	}

	return run.LatestRecord(ctx, db.cache)
}

// FindBySubRunRequestID finds a runstore record by request ID for a sub-DAG.
func (db *fileStore) FindBySubRunRequestID(ctx context.Context, reqID string, rootDAG digraph.RootDAG) (runstore.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindBySubRequestID canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if reqID == "" {
		return nil, ErrRequestIDEmpty
	}

	root := NewDataRoot(db.baseDir, rootDAG.Name)
	run, err := root.FindByRequestID(ctx, rootDAG.RequestID)
	if err != nil {
		return nil, err
	}

	subRun, err := run.FindSubRun(ctx, reqID)
	if err != nil {
		return nil, err
	}
	return subRun.LatestRecord(ctx, db.cache)
}

// RemoveOld removes runstore records older than retentionDays for the specified key.
func (db *fileStore) RemoveOld(ctx context.Context, dagName string, retentionDays int) error {
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

// Rename renames all runstore records from oldKey to newKey.
func (db *fileStore) Rename(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
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
