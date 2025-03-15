package jsondb

import (
	"context"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
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

// Repository returns a new HistoryData instance for the specified key.
func (db *JSONDB) Repository(ctx context.Context, dagName string) *Repository {
	return NewRepository(ctx, db.storage, db.baseDir, dagName, db.cache)
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (db *JSONDB) Update(ctx context.Context, dagName, requestID string, status persistence.Status) error {
	return db.Repository(ctx, dagName).Update(ctx, requestID, status)
}

// NewRecord creates a new history record for the specified key, timestamp, and request ID.
func (db *JSONDB) NewRecord(ctx context.Context, dagName string, timestamp time.Time, requestID string) persistence.Record {
	return db.Repository(ctx, dagName).NewRecord(ctx, timestamp, requestID)
}

// ReadRecent returns the most recent history records for the specified key, up to itemLimit.
func (db *JSONDB) ReadRecent(ctx context.Context, dagName string, itemLimit int) []persistence.Record {
	return db.Repository(ctx, dagName).Recent(ctx, itemLimit)
}

// ReadToday returns the most recent history record for today.
func (db *JSONDB) ReadToday(ctx context.Context, dagName string) (persistence.Record, error) {
	if db.latestStatusToday {
		return db.Repository(ctx, dagName).LatestToday(ctx)
	}
	return db.Repository(ctx, dagName).Latest(ctx)
}

// FindByRequestID finds a history record by request ID.
func (db *JSONDB) FindByRequestID(ctx context.Context, dagName string, requestID string) (persistence.Record, error) {
	return db.Repository(ctx, dagName).FindByRequestID(ctx, requestID)
}

// RemoveAll removes all history records for the specified key.
func (db *JSONDB) RemoveAll(ctx context.Context, dagName string) error {
	return db.RemoveOld(ctx, dagName, 0)
}

// RemoveOld removes history records older than retentionDays for the specified key.
func (db *JSONDB) RemoveOld(ctx context.Context, dagName string, retentionDays int) error {
	return db.Repository(ctx, dagName).RemoveOld(ctx, retentionDays)
}

// Rename renames all history records from oldKey to newKey.
func (db *JSONDB) Rename(ctx context.Context, oldPath, newPath string) error {
	return db.Repository(ctx, oldPath).Rename(ctx, newPath)
}
