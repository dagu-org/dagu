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
	"github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
)

// Error definitions for common issues
var (
	ErrRequestIDNotFound = errors.New("request ID not found")
	ErrInvalidPath       = errors.New("invalid path")
	ErrRequestIDEmpty    = errors.New("requestID is empty")
)

var _ persistence.HistoryStore = (*JSONDB)(nil)

// JSONDB manages DAGs status files in local storage with high performance and reliability.
type JSONDB struct {
	baseDir           string                                // Base directory for all status files
	latestStatusToday bool                                  // Whether to only return today's status
	cache             *filecache.Cache[*persistence.Status] // Optional cache for read operations
	maxWorkers        int                                   // Maximum number of parallel workers
	storage           storage.Storage                       // Storage interface for managing history records
}

// Option defines functional options for configuring JSONDB.
type Option func(*Options)

// Options holds configuration options for JSONDB.
type Options struct {
	FileCache         *filecache.Cache[*persistence.Status]
	LatestStatusToday bool
	MaxWorkers        int
	OperationTimeout  time.Duration
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
		storage:           storage.New(),
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

// NewRecord creates a new history record for the specified DAG execution.
func (db *JSONDB) NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string) persistence.Record {
	if reqID == "" {
		logger.Error(ctx, "RequestID is empty")
	}

	addr := storage.NewAddress(db.baseDir, dag.Name)
	filePath := db.storage.GenerateFilePath(ctx, addr, storage.NewUTC(timestamp), reqID)
	return NewRecord(filePath, db.cache, WithDAG(dag))
}

// NewSubRecord creates a new history record for the specified sub-DAG execution.
func (db *JSONDB) NewSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, rootReqID, reqID string) persistence.Record {
	if reqID == "" {
		logger.Error(ctx, "RequestID is empty")
	}
	if rootReqID == "" {
		logger.Error(ctx, "RootRequestID is empty")
	}

	addr := storage.NewAddress(db.baseDir, dag.Name)
	filePath := db.storage.GenerateFilePath(ctx, addr, storage.NewUTC(timestamp), reqID)
	return NewRecord(filePath, db.cache, WithDAG(dag))
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
	addr := storage.NewAddress(db.baseDir, dagName)
	files := db.storage.Latest(ctx, addr, itemLimit)
	if len(files) == 0 {
		return nil
	}

	// Create history records
	records := make([]persistence.Record, 0, len(files))
	for _, file := range files {
		records = append(records, NewRecord(file, db.cache))
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

	addr := storage.NewAddress(db.baseDir, dagName)

	if db.latestStatusToday {
		startOfDay := time.Now().Truncate(24 * time.Hour)
		startOfDayInUTC := storage.NewUTC(startOfDay)

		// Get the latest file for today
		file, err := db.storage.LatestAfter(ctx, addr, startOfDayInUTC)
		if err != nil {
			return nil, err
		}

		return NewRecord(file, db.cache), nil
	}

	// Get the latest file
	files := db.storage.Latest(ctx, addr, 1)
	if len(files) == 0 {
		return nil, persistence.ErrNoStatusData
	}
	return NewRecord(files[0], db.cache), nil
}

// FindByRequestID finds a history record by request ID.
func (db *JSONDB) FindByRequestID(ctx context.Context, dagName string, reqID string) (persistence.Record, error) {
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

	// Find matching files
	addr := storage.NewAddress(db.baseDir, dagName)
	file, err := db.storage.FindByRequestID(ctx, addr, reqID)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	// Return the most recent file
	return NewRecord(file, db.cache), nil
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

	addr := storage.NewAddress(db.baseDir, dagName)
	return db.storage.RemoveOld(ctx, addr, retentionDays)
}

// Rename renames all history records from oldKey to newKey.
func (db *JSONDB) Rename(ctx context.Context, oldPath, newNameOrPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	oldAddr := storage.NewAddress(db.baseDir, oldPath)
	newAddr := storage.NewAddress(db.baseDir, newNameOrPath)
	if err := db.storage.Rename(ctx, oldAddr, newAddr); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}
	return nil
}
