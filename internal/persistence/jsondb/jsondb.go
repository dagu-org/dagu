// Package jsondb provides a JSON-based database implementation for storing DAG execution history.
// It offers high-performance, thread-safe operations with metrics collection and caching support.
package jsondb

import (
	"context"
	"runtime"
	"sync"
	"time"

	// nolint: gosec
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// Error definitions for common issues
var (
	ErrRequestIDNotFound  = errors.New("request ID not found")
	ErrCreateNewDirectory = errors.New("failed to create new directory")
	ErrInvalidPath        = errors.New("invalid path")
	ErrKeyEmpty           = errors.New("key is empty")
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
	defaultWorkers    = 4
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
		OperationTimeout:  60 * time.Second,
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
func (db *JSONDB) Update(ctx context.Context, key, requestID string, status persistence.Status) error {
	// Validate inputs
	if key == "" {
		return ErrKeyEmpty
	}
	if requestID == "" {
		return ErrRequestIDEmpty
	}

	// Find the history record
	historyRecord, err := db.FindByRequestID(ctx, key, requestID)
	if err != nil {
		return fmt.Errorf("failed to find history record: %w", err)
	}

	// Open, write, and close the history record
	if err := historyRecord.Open(ctx); err != nil {
		return fmt.Errorf("failed to open history record: %w", err)
	}

	if err := historyRecord.Write(ctx, status); err != nil {
		// Try to close the record even if write fails
		closeErr := historyRecord.Close(ctx)
		if closeErr != nil {
			logger.Errorf(ctx, "Failed to close history record after write error: %v", closeErr)
		}
		return fmt.Errorf("failed to write status: %w", err)
	}

	if err := historyRecord.Close(ctx); err != nil {
		return fmt.Errorf("failed to close history record: %w", err)
	}
	return nil
}

// NewRecord creates a new history record for the specified key, timestamp, and request ID.
func (db *JSONDB) NewRecord(ctx context.Context, key string, timestamp time.Time, requestID string) persistence.HistoryRecord {
	// Validate inputs and log warnings for empty values
	if key == "" {
		logger.Error(ctx, "key is empty")
	}
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	filePath := db.generateFilePath(ctx, key, newUTC(timestamp), requestID)

	return NewHistoryRecord(filePath, db.cache)
}

// ReadRecent returns the most recent history records for the specified key, up to itemLimit.
func (db *JSONDB) ReadRecent(ctx context.Context, key string, itemLimit int) []persistence.HistoryRecord {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		logger.Errorf(ctx, "ReadRecent canceled: %v", ctx.Err())
		return nil
	default:
		// Continue with operation
	}

	// Validate inputs
	if key == "" {
		logger.Error(ctx, "key is empty")
		return nil
	}
	if itemLimit <= 0 {
		logger.Warnf(ctx, "Invalid itemLimit %d, using default of 10", itemLimit)
		itemLimit = 10
	}

	// Get the latest matches
	files := db.getLatestMatches(ctx, db.globPattern(key), itemLimit)
	if len(files) == 0 {
		logger.Debugf(ctx, "No recent records found for key %s", key)
		return nil
	}

	// Create history records
	records := make([]persistence.HistoryRecord, 0, len(files))
	for _, file := range files {
		records = append(records, NewHistoryRecord(file, db.cache))
	}

	return records
}

// ReadToday returns the most recent history record for today.
func (db *JSONDB) ReadToday(_ context.Context, key string) (persistence.HistoryRecord, error) {
	// Validate inputs
	if key == "" {
		return nil, ErrKeyEmpty
	}

	// Get the latest file for today
	file, err := db.latestToday(key, time.Now(), db.latestStatusToday)
	if err != nil {
		return nil, fmt.Errorf("failed to read status today for %s: %w", key, err)
	}

	return NewHistoryRecord(file, db.cache), nil
}

// FindByRequestID finds a history record by request ID.
func (db *JSONDB) FindByRequestID(_ context.Context, key string, requestID string) (persistence.HistoryRecord, error) {
	// Validate inputs
	if key == "" {
		return nil, ErrKeyEmpty
	}
	if requestID == "" {
		return nil, ErrRequestIDEmpty
	}

	// Find matching files
	matches, err := filepath.Glob(db.globPattern(key))
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	// Return the most recent file
	return NewHistoryRecord(matches[0], db.cache), nil
}

// RemoveAll removes all history records for the specified key.
func (db *JSONDB) RemoveAll(ctx context.Context, key string) error {
	return db.RemoveOld(ctx, key, 0)
}

// RemoveOld removes history records older than retentionDays for the specified key.
func (db *JSONDB) RemoveOld(ctx context.Context, key string, retentionDays int) error {
	// Validate inputs
	if key == "" {
		return ErrKeyEmpty
	}
	if retentionDays < 0 {
		logger.Warnf(ctx, "Negative retentionDays %d, no files will be removed", retentionDays)
		return nil
	}

	// Find matching files
	matches, err := filepath.Glob(db.globPattern(key))
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to remove for key %s", key)
		return nil
	}

	// Calculate the cutoff date
	oldDate := time.Now().AddDate(0, 0, -retentionDays)

	// Use a worker pool to remove files in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(matches))
	semaphore := make(chan struct{}, db.maxWorkers)

	for _, m := range matches {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Check if the file is older than the cutoff date
			info, err := os.Stat(filePath)
			if err != nil {
				logger.Debugf(ctx, "Failed to stat file %s: %v", filePath, err)
				return
			}

			if info.ModTime().Before(oldDate) {
				if err := os.Remove(filePath); err != nil {
					errChan <- fmt.Errorf("failed to remove file %s: %w", filePath, err)
				} else {
					logger.Debugf(ctx, "Removed old file %s", filePath)
				}
			}
		}(m)
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Rename renames all history records from oldKey to newKey.
func (db *JSONDB) Rename(ctx context.Context, oldKey, newKey string) error {
	// Validate inputs
	if oldKey == "" || newKey == "" {
		return ErrKeyEmpty
	}
	if !filepath.IsAbs(oldKey) || !filepath.IsAbs(newKey) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidPath, oldKey, newKey)
	}

	// Get the old directory
	oldDir := db.getDirectory(oldKey, getPrefix(oldKey))
	if !db.exists(oldDir) {
		logger.Debugf(ctx, "Old directory %s does not exist, nothing to rename", oldDir)
		return nil
	}

	// Create the new directory if it doesn't exist
	newDir := db.getDirectory(newKey, getPrefix(newKey))
	if !db.exists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", ErrCreateNewDirectory, newDir, err)
		}
	}

	// Find matching files
	matches, err := filepath.Glob(db.globPattern(oldKey))
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to rename for key %s", oldKey)
		return nil
	}

	// Get the old and new prefixes
	oldPrefix := filepath.Base(db.createPrefix(oldKey))
	newPrefix := filepath.Base(db.createPrefix(newKey))

	// Use a worker pool to rename files in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(matches))
	semaphore := make(chan struct{}, db.maxWorkers)

	for _, m := range matches {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Replace the old prefix with the new prefix
			base := filepath.Base(filePath)
			newName := strings.Replace(base, oldPrefix, newPrefix, 1)
			newPath := filepath.Join(newDir, newName)

			// Rename the file
			if err := os.Rename(filePath, newPath); err != nil {
				errChan <- fmt.Errorf("failed to rename %s to %s: %w", filePath, newPath, err)
				logger.Errorf(ctx, "Failed to rename %s to %s: %v", filePath, newPath, err)
			} else {
				logger.Debugf(ctx, "Renamed %s to %s", filePath, newPath)
			}
		}(m)
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Try to remove the old directory if it's empty
	if files, _ := os.ReadDir(oldDir); len(files) == 0 {
		if err := os.Remove(oldDir); err != nil {
			logger.Warnf(ctx, "Failed to remove empty directory %s: %v", oldDir, err)
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// getDirectory returns the directory for the specified key and prefix.
func (db *JSONDB) getDirectory(key string, prefix string) string {
	if key != prefix {
		// Add a hash postfix to the directory name to avoid conflicts.
		// nolint: gosec
		h := md5.New()
		_, _ = h.Write([]byte(key))
		v := hex.EncodeToString(h.Sum(nil))
		return filepath.Join(db.baseDir, fmt.Sprintf("%s-%s", prefix, v))
	}

	return filepath.Join(db.baseDir, key)
}

// generateFilePath generates a file path for the specified key, timestamp, and request ID.
func (db *JSONDB) generateFilePath(ctx context.Context, key string, timestamp timeInUTC, requestID string) string {
	if key == "" {
		logger.Error(ctx, "key is empty")
	}
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	prefix := db.createPrefix(key)
	timestampString := timestamp.Format(dateTimeFormatUTC)
	requestID = stringutil.TruncString(requestID, requestIDLenSafe)

	return fmt.Sprintf("%s.%s.%s.dat", prefix, timestampString, requestID)
}

// latestToday returns the path to the latest status file for today.
func (db *JSONDB) latestToday(key string, day time.Time, latestStatusToday bool) (string, error) {
	prefix := db.createPrefix(key)
	pattern := fmt.Sprintf("%s.*.*.dat", prefix)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusDataToday
	}

	ret := filterLatest(matches, 1, db.maxWorkers)
	if len(ret) == 0 {
		return "", persistence.ErrNoStatusData
	}

	startOfDay := day.Truncate(24 * time.Hour)
	startOfDayInUTC := newUTC(startOfDay)

	if latestStatusToday {
		timestamp, err := findTimestamp(ret[0])
		if err != nil {
			return "", err
		}
		if timestamp.Before(startOfDayInUTC.Time) {
			return "", persistence.ErrNoStatusDataToday
		}
	}

	return ret[0], nil
}

// getLatestMatches returns the latest matches for the specified pattern, up to itemLimit.
func (db *JSONDB) getLatestMatches(ctx context.Context, pattern string, itemLimit int) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Errorf(ctx, "Failed to find matches for pattern %s: %v", pattern, err)
		return nil
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No matches found for pattern %s", pattern)
		return nil
	}

	return filterLatest(matches, itemLimit, db.maxWorkers)
}

// globPattern returns the glob pattern for the specified key.
func (db *JSONDB) globPattern(key string) string {
	return db.createPrefix(key) + "*" + extDat
}

// createPrefix creates a prefix for the specified key.
func (db *JSONDB) createPrefix(key string) string {
	prefix := getPrefix(key)
	return filepath.Join(db.getDirectory(key, prefix), prefix)
}

// exists returns true if the specified file path exists.
func (db *JSONDB) exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
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

			t, err := findTimestamp(filePath)
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

// findTimestamp extracts and parses the timestamp from a file name.
func findTimestamp(file string) (time.Time, error) {
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

// getPrefix extracts the prefix from a key.
func getPrefix(key string) string {
	ext := filepath.Ext(key)
	if ext == "" {
		// No extension
		return filepath.Base(key)
	}
	if fileutil.IsYAMLFile(key) {
		// Remove .yaml or .yml extension
		return strings.TrimSuffix(filepath.Base(key), ext)
	}
	// Use the base name (if it's a path or just a name)
	return filepath.Base(key)
}

// timeInUTC is a wrapper for time.Time that ensures the time is in UTC.
type timeInUTC struct{ time.Time }

// newUTC creates a new timeInUTC from a time.Time.
func newUTC(t time.Time) timeInUTC {
	return timeInUTC{t.UTC()}
}
