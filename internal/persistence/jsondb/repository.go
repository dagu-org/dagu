// Package jsondb provides a JSON-based database implementation for storing DAG execution history.
package jsondb

import (
	"context" // nolint: gosec
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
	// "github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
)

// Error definitions for common issues
var (
	ErrRequestIDNotFound = errors.New("request ID not found")
	ErrInvalidPath       = errors.New("invalid path")
	ErrRequestIDEmpty    = errors.New("requestID is empty")
)

// Repository manages history records for a specific DAG, providing methods to create,
// read, update, and manage history files. It supports parallel processing for improved
// performance with large datasets.
type Repository struct {
	parentDir  string // Base directory for all history data
	addr       storage.Address
	maxWorkers int                                   // Maximum number of parallel workers
	cache      *filecache.Cache[*persistence.Status] // Optional cache for read operations
	storage    storage.Storage
}

// NewRepository creates a new HistoryData instance for the specified DAG.
// It normalizes the DAG name and sets up the appropriate directory structure.
func NewRepository(ctx context.Context, s storage.Storage, parentDir, dagName string, cache *filecache.Cache[*persistence.Status]) *Repository {
	if dagName == "" {
		logger.Error(ctx, "dagName is empty")
	}

	key := storage.NewAddress(parentDir, dagName)
	return &Repository{
		parentDir:  parentDir,
		addr:       key,
		cache:      cache,
		maxWorkers: runtime.NumCPU(),
		storage:    s,
	}
}

// NewRecord creates a new history record for the specified timestamp and request ID.
// The record is not opened or written to until explicitly requested.
func (r *Repository) NewRecord(ctx context.Context, timestamp time.Time, requestID string) persistence.Record {
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	filePath := r.storage.GenerateFilePath(ctx, r.addr, storage.NewUTC(timestamp), requestID)
	return NewRecord(filePath, r.cache)
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (r *Repository) Update(ctx context.Context, requestID string, status persistence.Status) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if requestID == "" {
		return ErrRequestIDEmpty
	}

	// Find the history record
	historyRecord, err := r.FindByRequestID(ctx, requestID)
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

// Rename renames all history records from the current DAG name to a new path.
// It creates the new directory structure and moves all matching files.
func (r *Repository) Rename(ctx context.Context, newNameOrPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	newAddr := storage.NewAddress(r.parentDir, newNameOrPath)
	if err := r.storage.Rename(ctx, r.addr, newAddr); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	r.addr = newAddr
	return nil
}

// Recent returns the most recent history records up to itemLimit.
// Records are sorted by timestamp with the most recent first.
func (r *Repository) Recent(ctx context.Context, itemLimit int) []persistence.Record {
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
	files := r.storage.Latest(ctx, r.addr, itemLimit)
	if len(files) == 0 {
		return nil
	}

	// Create history records
	records := make([]persistence.Record, 0, len(files))
	for _, file := range files {
		records = append(records, NewRecord(file, r.cache))
	}

	return records
}

// LatestToday returns the most recent history record for today.
// If no records exist for today, it returns an error.
func (r *Repository) LatestToday(ctx context.Context) (persistence.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("LatestToday canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	startOfDay := time.Now().Truncate(24 * time.Hour)
	startOfDayInUTC := storage.NewUTC(startOfDay)

	// Get the latest file for today
	file, err := r.storage.LatestAfter(ctx, r.addr, startOfDayInUTC)
	if err != nil {
		return nil, err
	}

	return NewRecord(file, r.cache), nil
}

// Latest returns the most recent history record regardless of date.
// If no records exist, it returns an error.
func (r *Repository) Latest(ctx context.Context) (persistence.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Latest canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Get the latest file
	files := r.storage.Latest(ctx, r.addr, 1)
	if len(files) == 0 {
		return nil, persistence.ErrNoStatusData
	}
	return NewRecord(files[0], r.cache), nil
}

// FindByRequestID finds a history record by request ID.
// It returns the most recent record if multiple matches exist.
func (r *Repository) FindByRequestID(ctx context.Context, requestID string) (persistence.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindByRequestID canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if requestID == "" {
		return nil, ErrRequestIDEmpty
	}

	// Find matching files
	file, err := r.storage.FindByRequestID(ctx, r.addr, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	// Return the most recent file
	return NewRecord(file, r.cache), nil
}

// RemoveOld removes history records older than retentionDays.
// It uses parallel processing for improved performance with large datasets.
func (r *Repository) RemoveOld(ctx context.Context, retentionDays int) error {
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

	return r.storage.RemoveOld(ctx, r.addr, retentionDays)
}
