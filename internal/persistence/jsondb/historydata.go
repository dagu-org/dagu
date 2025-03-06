// Package jsondb provides a JSON-based database implementation for storing DAG execution history.
package jsondb

import (
	"context"
	"crypto/md5" // nolint: gosec
	"encoding/hex"
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

// HistoryData manages history records for a specific DAG, providing methods to create,
// read, update, and manage history files. It supports parallel processing for improved
// performance with large datasets.
type HistoryData struct {
	parentDir  string                                // Base directory for all history data
	baseDir    string                                // Directory specific to this DAG
	dagName    string                                // Original DAG name/path
	key        string                                // Normalized key derived from dagName
	maxWorkers int                                   // Maximum number of parallel workers
	cache      *filecache.Cache[*persistence.Status] // Optional cache for read operations
}

// NewHistoryData creates a new HistoryData instance for the specified DAG.
// It normalizes the DAG name and sets up the appropriate directory structure.
func NewHistoryData(ctx context.Context, parentDir, dagName string, cache *filecache.Cache[*persistence.Status]) *HistoryData {
	if dagName == "" {
		logger.Error(ctx, "dagName is empty")
	}

	key := normalizeKey(dagName)
	return &HistoryData{
		parentDir:  parentDir,
		dagName:    dagName,
		baseDir:    getDirectory(parentDir, dagName, key),
		key:        key,
		cache:      cache,
		maxWorkers: runtime.NumCPU(),
	}
}

// NewRecord creates a new history record for the specified timestamp and request ID.
// The record is not opened or written to until explicitly requested.
func (hd *HistoryData) NewRecord(ctx context.Context, timestamp time.Time, requestID string) persistence.Record {
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	filePath := hd.generateFilePath(ctx, newUTC(timestamp), requestID)
	return NewRecord(filePath, hd.cache)
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (hd *HistoryData) Update(ctx context.Context, requestID string, status persistence.Status) error {
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
	historyRecord, err := hd.FindByRequestID(ctx, requestID)
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
func (hd *HistoryData) Rename(ctx context.Context, newPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if !filepath.IsAbs(hd.dagName) || !filepath.IsAbs(newPath) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidPath, hd.dagName, newPath)
	}

	// Get the old directory
	oldDir := hd.baseDir
	if !hd.exists(oldDir) {
		logger.Debugf(ctx, "Old directory %s does not exist, nothing to rename", oldDir)
		return nil
	}

	// Create the new directory if it doesn't exist
	newKey := normalizeKey(newPath)
	newBaseDir := getDirectory(hd.parentDir, newPath, newKey)
	if !hd.exists(newBaseDir) {
		if err := os.MkdirAll(newBaseDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", ErrCreateNewDirectory, newBaseDir, err)
		}
	}

	// Find matching files
	matches, err := filepath.Glob(hd.globPattern())
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to rename for key %s", hd.dagName)
		return nil
	}

	// Process files in parallel
	errs := hd.processFilesParallel(ctx, matches, func(filePath string) error {
		// Replace the old prefix with the new prefix
		base := filepath.Base(filePath)
		newName := strings.Replace(base, hd.key, newKey, 1)
		newFilePath := filepath.Join(newBaseDir, newName)

		// Rename the file
		if err := os.Rename(filePath, newFilePath); err != nil {
			logger.Errorf(ctx, "Failed to rename %s to %s: %v", filePath, newFilePath, err)
			return fmt.Errorf("failed to rename %s to %s: %w", filePath, newFilePath, err)
		}

		logger.Debugf(ctx, "Renamed %s to %s", filePath, newFilePath)
		return nil
	})

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

	// Update the instance properties
	hd.baseDir = newBaseDir
	hd.dagName = newPath
	hd.key = newKey

	return nil
}

// Recent returns the most recent history records up to itemLimit.
// Records are sorted by timestamp with the most recent first.
func (hd *HistoryData) Recent(ctx context.Context, itemLimit int) []persistence.Record {
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
	files := hd.getLatestMatches(ctx, hd.globPattern(), itemLimit)
	if len(files) == 0 {
		return nil
	}

	// Create history records
	records := make([]persistence.Record, 0, len(files))
	for _, file := range files {
		records = append(records, NewRecord(file, hd.cache))
	}

	return records
}

// LatestToday returns the most recent history record for today.
// If no records exist for today, it returns an error.
func (hd *HistoryData) LatestToday(ctx context.Context) (persistence.Record, error) {
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
	file, err := hd.latest(startOfDayInUTC)
	if err != nil {
		return nil, err
	}

	return NewRecord(file, hd.cache), nil
}

// Latest returns the most recent history record regardless of date.
// If no records exist, it returns an error.
func (hd *HistoryData) Latest(ctx context.Context) (persistence.Record, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Latest canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Get the latest file
	file, err := hd.latest(timeInUTC{})
	if err != nil {
		return nil, err
	}

	return NewRecord(file, hd.cache), nil
}

// FindByRequestID finds a history record by request ID.
// It returns the most recent record if multiple matches exist.
func (hd *HistoryData) FindByRequestID(ctx context.Context, requestID string) (persistence.Record, error) {
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
	matches, err := filepath.Glob(hd.globPattern())
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	// Return the most recent file
	return NewRecord(matches[0], hd.cache), nil
}

// RemoveOld removes history records older than retentionDays.
// It uses parallel processing for improved performance with large datasets.
func (hd *HistoryData) RemoveOld(ctx context.Context, retentionDays int) error {
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
	matches, err := filepath.Glob(hd.globPattern())
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil
	}

	// Calculate the cutoff date
	oldDate := time.Now().AddDate(0, 0, -retentionDays)

	// Process files in parallel
	errs := hd.processFilesParallel(ctx, matches, func(filePath string) error {
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
func (hd *HistoryData) processFilesParallel(ctx context.Context, files []string, processor func(string) error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, hd.maxWorkers)

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
func (hd *HistoryData) latest(cutoff timeInUTC) (string, error) {
	pattern := path.Join(hd.baseDir, hd.key+"*"+extDat)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusData
	}

	ret := filterLatest(matches, 1, hd.maxWorkers)
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
func (hd *HistoryData) getLatestMatches(ctx context.Context, pattern string, itemLimit int) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Errorf(ctx, "Failed to find matches for pattern %s: %v", pattern, err)
		return nil
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No matches found for pattern %s", pattern)
		return nil
	}

	return filterLatest(matches, itemLimit, hd.maxWorkers)
}

// globPattern returns the glob pattern for finding history files for this DAG.
func (hd *HistoryData) globPattern() string {
	return path.Join(hd.baseDir, hd.key+"*"+extDat)
}

// generateFilePath generates a file path for a history record.
func (hd *HistoryData) generateFilePath(ctx context.Context, timestamp timeInUTC, reqID string) string {
	if reqID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	ts := timestamp.Format(dateTimeFormatUTC)
	reqID = stringutil.TruncString(reqID, requestIDLenSafe)

	return path.Join(hd.baseDir, fmt.Sprintf("%s.%s.%s%s", hd.key, ts, reqID, extDat))
}

// exists returns true if the specified file path exists.
func (hd *HistoryData) exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// getDirectory returns the directory path for storing history files.
// It handles cases where the original key needs to be hashed to avoid conflicts.
func getDirectory(baseDir, originalKey, normalizedKey string) string {
	if originalKey != normalizedKey {
		// Add a hash postfix to the directory name to avoid conflicts.
		// nolint: gosec
		h := md5.New()
		_, _ = h.Write([]byte(originalKey))
		v := hex.EncodeToString(h.Sum(nil))
		return filepath.Join(baseDir, fmt.Sprintf("%s-%s", normalizedKey, v))
	}

	return filepath.Join(baseDir, originalKey)
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

// normalizeKey normalizes the key by removing the extension and directory path.
func normalizeKey(key string) string {
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
