package jsondb

import (
	"context"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

var _ persistence.HistoryStore = (*JsonDB)(nil)

// JsonDB manages DAGs status files in local storage with high performance and reliability.
type JsonDB struct {
	baseDir           string                                // Base directory for all status files
	latestStatusToday bool                                  // Whether to only return today's status
	cache             *filecache.Cache[*persistence.Status] // Optional cache for read operations
	maxWorkers        int                                   // Maximum number of parallel workers
}

// Option defines functional options for configuring JsonDB.
type Option func(*Options)

// Options holds configuration options for JsonDB.
type Options struct {
	FileCache         *filecache.Cache[*persistence.Status]
	LatestStatusToday bool
	MaxWorkers        int
	OperationTimeout  time.Duration
}

// WithFileCache sets the file cache for JsonDB.
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

// New creates a new JsonDB instance with the specified options.
func New(baseDir string, opts ...Option) *JsonDB {
	options := &Options{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &JsonDB{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// Data returns a new HistoryData instance for the specified key.
func (db *JsonDB) Data(ctx context.Context, dagName string) *Repository {
	return NewRepository(ctx, db.baseDir, dagName, db.cache)
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (db *JsonDB) Update(ctx context.Context, dagName, requestID string, status persistence.Status) error {
	return db.Data(ctx, dagName).Update(ctx, requestID, status)
}

// NewRecord creates a new history record for the specified key, timestamp, and request ID.
func (db *JsonDB) NewRecord(ctx context.Context, dagName string, timestamp time.Time, requestID string) persistence.Record {
	return db.Data(ctx, dagName).NewRecord(ctx, timestamp, requestID)
}

// ReadRecent returns the most recent history records for the specified key, up to itemLimit.
func (db *JsonDB) ReadRecent(ctx context.Context, dagName string, itemLimit int) []persistence.Record {
	return db.Data(ctx, dagName).Recent(ctx, itemLimit)
}

// ReadToday returns the most recent history record for today.
func (db *JsonDB) ReadToday(ctx context.Context, dagName string) (persistence.Record, error) {
	if db.latestStatusToday {
		return db.Data(ctx, dagName).LatestToday(ctx)
	}
	return db.Data(ctx, dagName).Latest(ctx)
}

// FindByRequestID finds a history record by request ID.
func (db *JsonDB) FindByRequestID(ctx context.Context, dagName string, requestID string) (persistence.Record, error) {
	return db.Data(ctx, dagName).FindByRequestID(ctx, requestID)
}

// RemoveAll removes all history records for the specified key.
func (db *JsonDB) RemoveAll(ctx context.Context, dagName string) error {
	return db.RemoveOld(ctx, dagName, 0)
}

// RemoveOld removes history records older than retentionDays for the specified key.
func (db *JsonDB) RemoveOld(ctx context.Context, dagName string, retentionDays int) error {
	return db.Data(ctx, dagName).RemoveOld(ctx, retentionDays)
}

// Rename renames all history records from oldKey to newKey.
func (db *JsonDB) Rename(ctx context.Context, oldPath, newPath string) error {
	return db.Data(ctx, oldPath).Rename(ctx, newPath)
}
