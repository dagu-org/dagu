// Package jsondb provides a JSON-based database implementation for storing DAG execution history.
package jsondb

import (
	"context" // nolint: gosec
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// Error definitions for common issues
var (
	ErrRequestIDNotFound  = errors.New("request ID not found")
	ErrCreateNewDirectory = errors.New("failed to create new directory")
	ErrRemoveDirectory    = errors.New("failed to remove directory")
	ErrMoveDirectory      = errors.New("failed to move directory")
	ErrInvalidPath        = errors.New("invalid path")
	ErrRequestIDEmpty     = errors.New("requestID is empty")

	// rTimestamp is a regular expression to match the timestamp in the file name.
	rTimestamp = regexp.MustCompile(`2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}|2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}Z`)
)

// Constants for file naming and formatting
const (
	requestIDLenSafe  = 8
	extDat            = ".dat"
	dateTimeFormatUTC = "20060102.15:04:05.000Z"
	dateTimeFormat    = "20060102.15:04:05.000"
	dateFormat        = "20060102"
)

// Repository manages history records for a specific DAG, providing methods to create,
// read, update, and manage history files. It supports parallel processing for improved
// performance with large datasets.
type Repository struct {
	parentDir  string // Base directory for all history data
	addr       StorageAddress
	maxWorkers int                                   // Maximum number of parallel workers
	cache      *filecache.Cache[*persistence.Status] // Optional cache for read operations
}

// NewRepository creates a new HistoryData instance for the specified DAG.
// It normalizes the DAG name and sets up the appropriate directory structure.
func NewRepository(ctx context.Context, parentDir, dagName string, cache *filecache.Cache[*persistence.Status]) *Repository {
	if dagName == "" {
		logger.Error(ctx, "dagName is empty")
	}

	key := NewStorageAddress(parentDir, dagName)
	return &Repository{
		parentDir:  parentDir,
		addr:       key,
		cache:      cache,
		maxWorkers: runtime.NumCPU(),
	}
}

// NewRecord creates a new history record for the specified timestamp and request ID.
// The record is not opened or written to until explicitly requested.
func (r *Repository) NewRecord(ctx context.Context, timestamp time.Time, requestID string) persistence.Record {
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	filePath := r.generateFilePath(ctx, newUTC(timestamp), requestID)
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

	// Get the old directory
	if !r.addr.Exists() {
		return nil
	}

	// Create the new directory if it doesn't exist
	newAddr := NewStorageAddress(r.parentDir, newNameOrPath)
	if !newAddr.Exists() {
		if err := newAddr.Create(); err != nil {
			return err
		}
	}

	// Find matching files
	matches, err := filepath.Glob(r.globPattern())
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to rename for %q", r.globPattern())
		return nil
	}

	// Process files in parallel
	errs := r.processFilesParallel(ctx, matches, func(filePath string) error {
		// Replace the old prefix with the new prefix
		base := filepath.Base(filePath)
		newName := strings.Replace(base, r.addr.prefix, newAddr.prefix, 1)
		newFilePath := filepath.Join(newAddr.path, newName)

		// Rename the file
		if err := os.Rename(filePath, newFilePath); err != nil {
			logger.Errorf(ctx, "Failed to rename %s to %s: %v", filePath, newFilePath, err)
			return fmt.Errorf("failed to rename %s to %s: %w", filePath, newFilePath, err)
		}

		logger.Debugf(ctx, "Renamed %s to %s", filePath, newFilePath)
		return nil
	})

	// Try to remove the old directory if it's empty
	if r.addr.IsEmpty() {
		if err := r.addr.Remove(); err != nil {
			logger.Warn(ctx, "Failed to remove old directory", "err", err)
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
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
	files := r.getLatestMatches(ctx, r.globPattern(), itemLimit)
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
	startOfDayInUTC := newUTC(startOfDay)

	// Get the latest file for today
	file, err := r.latest(startOfDayInUTC)
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
	file, err := r.latest(timeInUTC{})
	if err != nil {
		return nil, err
	}

	return NewRecord(file, r.cache), nil
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
	matches, err := filepath.Glob(r.globPattern())
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	// Return the most recent file
	return NewRecord(matches[0], r.cache), nil
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

	// Find matching files
	matches, err := filepath.Glob(r.globPattern())
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil
	}

	// Calculate the cutoff date
	oldDate := time.Now().AddDate(0, 0, -retentionDays)

	// Process files in parallel
	errs := r.processFilesParallel(ctx, matches, func(filePath string) error {
		// Check if the file is older than the cutoff date
		info, err := os.Stat(filePath)
		if err != nil {
			logger.Debugf(ctx, "Failed to stat file %s: %v", filePath, err)
			return nil // Skip files we can't stat
		}

		if info.ModTime().Before(oldDate) {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove file %s: %w", filePath, err)
			}
			logger.Debugf(ctx, "Removed old file %s", filePath)
		}
		return nil
	})

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// processFilesParallel processes files in parallel using a worker pool.
// It returns a slice of errors encountered during processing.
func (r *Repository) processFilesParallel(ctx context.Context, files []string, processor func(string) error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, r.maxWorkers)

	for _, file := range files {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return []error{fmt.Errorf("operation canceled: %w", ctx.Err())}
		default:
			// Continue processing
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if err := processor(filePath); err != nil {
				errChan <- err
			}
		}(file)
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	return errs
}

// latest returns the path to the latest history record file.
// If cutoff is not zero, it only returns files newer than the cutoff time.
func (r *Repository) latest(cutoff timeInUTC) (string, error) {
	pattern := path.Join(r.addr.path, r.addr.prefix+"*"+extDat)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusData
	}

	ret := filterLatest(matches, 1, r.maxWorkers)
	if len(ret) == 0 {
		return "", persistence.ErrNoStatusData
	}

	if cutoff.IsZero() {
		return ret[0], nil
	}

	timestamp, err := parseFileTimestamp(ret[0])
	if err != nil {
		return "", fmt.Errorf("failed to parse timestamp: %w", err)
	}
	if timestamp.Before(cutoff.Time) {
		return "", persistence.ErrNoStatusData
	}

	return ret[0], nil
}

// getLatestMatches returns the latest matches for the specified pattern, up to itemLimit.
func (r *Repository) getLatestMatches(ctx context.Context, pattern string, itemLimit int) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Errorf(ctx, "Failed to find matches for pattern %s: %v", pattern, err)
		return nil
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No matches found for pattern %s", pattern)
		return nil
	}

	return filterLatest(matches, itemLimit, r.maxWorkers)
}

// globPattern returns the glob pattern for finding history files for this DAG.
func (r *Repository) globPattern() string {
	return path.Join(r.addr.path, r.addr.prefix+"*"+extDat)
}

// generateFilePath generates a file path for a history record.
func (r *Repository) generateFilePath(ctx context.Context, timestamp timeInUTC, reqID string) string {
	if reqID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	ts := timestamp.Format(dateTimeFormatUTC)
	reqID = stringutil.TruncString(reqID, requestIDLenSafe)

	return path.Join(r.addr.path, fmt.Sprintf("%s.%s.%s%s", r.addr.prefix, ts, reqID, extDat))
}

// filterLatest returns the most recent files up to itemLimit
// Uses parallel processing for large file sets to improve performance
func filterLatest(files []string, itemLimit int, maxWorkers int) []string {
	if len(files) == 0 {
		return nil
	}

	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	// Pre-compute timestamps to avoid repeated regex operations
	type fileWithTime struct {
		path string
		time time.Time
		err  error
	}

	filesWithTime := make([]fileWithTime, len(files))

	// Process files in parallel with worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)

	for i, file := range files {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(idx int, filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			t, err := parseFileTimestamp(filePath)
			filesWithTime[idx] = fileWithTime{filePath, t, err}
		}(i, file)
	}

	wg.Wait()

	// Sort by timestamp (most recent first)
	sort.Slice(filesWithTime, func(i, j int) bool {
		// Files with errors go to the end
		if filesWithTime[i].err != nil {
			return false
		}
		if filesWithTime[j].err != nil {
			return true
		}
		return filesWithTime[i].time.After(filesWithTime[j].time)
	})

	// Extract just the paths, limiting to requested count
	// Pre-allocate with exact capacity for efficiency
	limit := min(len(filesWithTime), itemLimit)
	result := make([]string, 0, limit)

	for i := 0; i < limit; i++ {
		if filesWithTime[i].err == nil {
			result = append(result, filesWithTime[i].path)
		}
	}

	return result
}

// parseFileTimestamp extracts and parses the timestamp from a file name.
func parseFileTimestamp(file string) (time.Time, error) {
	timestampString := rTimestamp.FindString(file)
	if timestampString == "" {
		return time.Time{}, fmt.Errorf("no timestamp found in file name: %s", file)
	}

	if !strings.Contains(timestampString, "Z") {
		// For backward compatibility
		t, err := time.Parse(dateTimeFormat, timestampString)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse timestamp %s: %w", timestampString, err)
		}
		return t, nil
	}

	// UTC
	t, err := time.Parse(dateTimeFormatUTC, timestampString)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", timestampString, err)
	}
	return t, nil
}

// timeInUTC is a wrapper for time.Time that ensures the time is in UTC.
type timeInUTC struct{ time.Time }

// newUTC creates a new timeInUTC from a time.Time.
func newUTC(t time.Time) timeInUTC {
	return timeInUTC{t.UTC()}
}
