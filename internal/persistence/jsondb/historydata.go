package jsondb

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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

type HistoryData struct {
	baseDir    string
	dagName    string
	maxWorkers int                                   // Maximum number of parallel workers
	cache      *filecache.Cache[*persistence.Status] // Optional cache for read operations
}

func NewHistoryData(ctx context.Context, baseDir, dagName string, cache *filecache.Cache[*persistence.Status]) *HistoryData {
	if dagName == "" {
		logger.Error(ctx, "dagName is empty")
	}
	return &HistoryData{
		baseDir:    baseDir,
		dagName:    dagName,
		cache:      cache,
		maxWorkers: runtime.NumCPU(),
	}
}

// NewRecord creates a new history record for the specified key, timestamp, and request ID.
func (hd *HistoryData) NewRecord(ctx context.Context, timestamp time.Time, requestID string) persistence.HistoryRecord {
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	filePath := hd.generateFilePath(ctx, newUTC(timestamp), requestID)
	return NewHistoryRecord(filePath, hd.cache)
}

// Update updates the status for a specific request ID.
// It handles the entire lifecycle of opening, writing, and closing the history record.
func (db *HistoryData) Update(ctx context.Context, requestID string, status persistence.Status) error {
	if requestID == "" {
		return ErrRequestIDEmpty
	}

	// Find the history record
	historyRecord, err := db.FindByRequestID(ctx, requestID)
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

// Rename renames all history records from oldKey to newKey.
func (hd *HistoryData) Rename(ctx context.Context, newPath string) error {
	if !filepath.IsAbs(hd.dagName) || !filepath.IsAbs(newPath) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidPath, hd.dagName, newPath)
	}

	// Get the old directory
	oldDir := hd.getDirectory(hd.dagName, getPrefix(hd.dagName))
	if !hd.exists(oldDir) {
		logger.Debugf(ctx, "Old directory %s does not exist, nothing to rename", oldDir)
		return nil
	}

	// Create the new directory if it doesn't exist
	newDir := hd.getDirectory(newPath, getPrefix(newPath))
	if !hd.exists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", ErrCreateNewDirectory, newDir, err)
		}
	}

	// Find matching files
	matches, err := filepath.Glob(hd.globPattern(hd.dagName))
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to rename for key %s", hd.dagName)
		return nil
	}

	// Get the old and new prefixes
	oldPrefix := filepath.Base(hd.createPrefix(hd.dagName))
	newPrefix := filepath.Base(hd.createPrefix(newPath))

	// Use a worker pool to rename files in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(matches))
	semaphore := make(chan struct{}, hd.maxWorkers)

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

// ReadRecent returns the most recent history records for the specified key, up to itemLimit.
func (hd *HistoryData) Recent(ctx context.Context, itemLimit int) []persistence.HistoryRecord {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		logger.Errorf(ctx, "ReadRecent canceled: %v", ctx.Err())
		return nil
	default:
		// Continue with operation
	}

	if itemLimit <= 0 {
		logger.Warnf(ctx, "Invalid itemLimit %d, using default of 10", itemLimit)
		itemLimit = 10
	}

	// Get the latest matches
	files := hd.getLatestMatches(ctx, hd.globPattern(hd.dagName), itemLimit)
	if len(files) == 0 {
		return nil
	}

	// Create history records
	records := make([]persistence.HistoryRecord, 0, len(files))
	for _, file := range files {
		records = append(records, NewHistoryRecord(file, hd.cache))
	}

	return records
}

// LatestToday returns the most recent history record for today.
func (hd *HistoryData) LatestToday(_ context.Context) (persistence.HistoryRecord, error) {
	startOfDay := time.Now().Truncate(24 * time.Hour)
	startOfDayInUTC := newUTC(startOfDay)

	// Get the latest file for today
	file, err := hd.latest(startOfDayInUTC)
	if err != nil {
		return nil, err
	}

	return NewHistoryRecord(file, hd.cache), nil
}

// Latest returns the most recent history record.
func (hd *HistoryData) Latest(_ context.Context) (persistence.HistoryRecord, error) {
	// Get the latest file
	file, err := hd.latest(timeInUTC{})
	if err != nil {
		return nil, err
	}

	return NewHistoryRecord(file, hd.cache), nil
}

// latest returns the latest history record for the specified key.
func (hd *HistoryData) latest(cutoff timeInUTC) (string, error) {
	prefix := hd.createPrefix(hd.dagName)
	pattern := fmt.Sprintf("%s.*.*.dat", prefix)

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

// FindByRequestID finds a history record by request ID.
func (hd *HistoryData) FindByRequestID(_ context.Context, requestID string) (persistence.HistoryRecord, error) {
	if requestID == "" {
		return nil, ErrRequestIDEmpty
	}

	// Find matching files
	matches, err := filepath.Glob(hd.globPattern(hd.dagName))
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	// Return the most recent file
	return NewHistoryRecord(matches[0], hd.cache), nil
}

// RemoveOld removes history records older than retentionDays for the specified key.
func (hd *HistoryData) RemoveOld(ctx context.Context, retentionDays int) error {
	if retentionDays < 0 {
		logger.Warnf(ctx, "Negative retentionDays %d, no files will be removed", retentionDays)
		return nil
	}

	// Find matching files
	matches, err := filepath.Glob(hd.globPattern(hd.dagName))
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil
	}

	// Calculate the cutoff date
	oldDate := time.Now().AddDate(0, 0, -retentionDays)

	// Use a worker pool to remove files in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(matches))
	semaphore := make(chan struct{}, hd.maxWorkers)

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

// globPattern returns the glob pattern for the specified key.
func (hd *HistoryData) globPattern(key string) string {
	return hd.createPrefix(key) + "*" + extDat
}

// createPrefix creates a prefix for the specified key.
func (hd *HistoryData) createPrefix(key string) string {
	prefix := getPrefix(key)
	return filepath.Join(hd.getDirectory(key, prefix), prefix)
}

// getDirectory returns the directory for the specified key and prefix.
func (hd *HistoryData) getDirectory(key string, prefix string) string {
	if key != prefix {
		// Add a hash postfix to the directory name to avoid conflicts.
		// nolint: gosec
		h := md5.New()
		_, _ = h.Write([]byte(key))
		v := hex.EncodeToString(h.Sum(nil))
		return filepath.Join(hd.baseDir, fmt.Sprintf("%s-%s", prefix, v))
	}

	return filepath.Join(hd.baseDir, key)
}

// generateFilePath generates a file path for the specified key, timestamp, and request ID.
func (hd *HistoryData) generateFilePath(ctx context.Context, timestamp timeInUTC, requestID string) string {
	if requestID == "" {
		logger.Error(ctx, "requestID is empty")
	}

	prefix := hd.createPrefix(hd.dagName)
	timestampString := timestamp.Format(dateTimeFormatUTC)
	requestID = stringutil.TruncString(requestID, requestIDLenSafe)

	return fmt.Sprintf("%s.%s.%s.dat", prefix, timestampString, requestID)
}

// exists returns true if the specified file path exists.
func (hd *HistoryData) exists(filePath string) bool {
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
