package jsondb

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

// Error definitions for common issues
var (
	ErrInvalidPath        = errors.New("invalid path")
	ErrRequestIDEmpty     = errors.New("requestID is empty")
	ErrRootRequestIDEmpty = errors.New("root requestID is empty")
)

var _ persistence.HistoryStore = (*JSONDB)(nil)

// JSONDB manages DAGs status files in local storage with high performance and reliability.
type JSONDB struct {
	baseDir           string                                // Base directory for all status files
	latestStatusToday bool                                  // Whether to only return today's status
	cache             *filecache.Cache[*persistence.Status] // Optional cache for read operations
	maxWorkers        int                                   // Maximum number of parallel workers
}

// Option defines functional options for configuring JSONDB.
type Option func(*Options)

// Options holds configuration options for JSONDB.
type Options struct {
	FileCache         *filecache.Cache[*persistence.Status] // Optional cache for status files
	LatestStatusToday bool                                  // Whether to only return today's status
	MaxWorkers        int                                   // Maximum number of parallel workers
	OperationTimeout  time.Duration                         // Timeout for operations
}

// WithFileCache sets the file cache for JSONDB.
func WithFileCache(cache *filecache.Cache[*persistence.Status]) Option {
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
func New(baseDir string, opts ...Option) *JSONDB {
	options := &Options{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &JSONDB{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (db *JSONDB) Update(ctx context.Context, dagName, reqID string, status persistence.Status) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if reqID == "" {
		return ErrRequestIDEmpty
	}

	// Find the history record
	historyRecord, err := db.FindByRequestID(ctx, dagName, reqID)
	if err != nil {
		return fmt.Errorf("failed to find history record: %w", err)
	}

	// Open, write, and close the history record
	if err := historyRecord.Open(ctx); err != nil {
		return fmt.Errorf("failed to open history record: %w", err)
	}

	// Ensure the record is closed even if write fails
	defer func() {
		if closeErr := historyRecord.Close(ctx); closeErr != nil {
			logger.Errorf(ctx, "Failed to close history record: %v", closeErr)
		}
	}()

	if err := historyRecord.Write(ctx, status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// NewRecord creates a new history record for the specified DAG run.
// If opts.Root is not nil, it creates a sub-record for the specified root DAG.
// If opts.Retry is true, it creates a retry record for the specified request ID.
func (db *JSONDB) NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts persistence.NewRecordOptions) (persistence.Record, error) {
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

// NewSubRecord creates a new history record for the specified sub-run.
func (db *JSONDB) newSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts persistence.NewRecordOptions) (persistence.Record, error) {
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

// Recent returns the most recent history records for the specified key, up to itemLimit.
func (db *JSONDB) Recent(ctx context.Context, dagName string, itemLimit int) []persistence.Record {
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
	records := make([]persistence.Record, 0, len(items))
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

// Latest returns the most recent history record for today.
func (db *JSONDB) Latest(ctx context.Context, dagName string) (persistence.Record, error) {
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
		return nil, persistence.ErrNoStatusData
	}
	return latestRun[0].LatestRecord(ctx, db.cache)
}

// FindByRequestID finds a history record by request ID.
func (db *JSONDB) FindByRequestID(ctx context.Context, dagName, reqID string) (persistence.Record, error) {
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

// FindBySubRequestID finds a history record by request ID for a sub-DAG.
func (db *JSONDB) FindBySubRequestID(ctx context.Context, reqID string, rootDAG digraph.RootDAG) (persistence.Record, error) {
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

// RemoveOld removes history records older than retentionDays for the specified key.
func (db *JSONDB) RemoveOld(ctx context.Context, dagName string, retentionDays int) error {
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

// Rename renames all history records from oldKey to newKey.
func (db *JSONDB) Rename(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
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
