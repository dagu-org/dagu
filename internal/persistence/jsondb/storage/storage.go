package storage

import (
	"context"
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

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
)

var maxWorkers = runtime.NumCPU()

// dataFileExtension is the file extension for history record files.
var dataFileExtension = ".dat"

// rTimestamp is a regular expression to match the timestamp in the file name.
var rTimestamp = regexp.MustCompile(`2\d{7}_\d{6}_\d{3}Z`)

// Filename formats
const (
	dateTimeFormatUTC = "20060102_150405_000Z"
	dateFormat        = "20060102"
)

// TimeInUTC is a wrapper for time.Time that ensures the time is in UTC.
type TimeInUTC struct{ time.Time }

// NewUTC creates a new timeInUTC from a time.Time.
func NewUTC(t time.Time) TimeInUTC {
	return TimeInUTC{t.UTC()}
}

type Storage interface {
	// Latest returns the latest history record files for the specified address, up to itemLimit.
	Latest(ctx context.Context, a Address, itemLimit int) []string
	// LatestAfter returns the path to the latest history record file.
	LatestAfter(ctx context.Context, a Address, cutoff TimeInUTC) (string, error)
	// GenerateFilePath generates a file path for a history record.
	GenerateFilePath(ctx context.Context, a Address, timestamp TimeInUTC, reqID string) string
	// Rename renames a file from the old address to the new address.
	Rename(ctx context.Context, o, n Address) error
	// RemoveOld removes history records older than retentionDays.
	// It uses parallel processing for improved performance with large datasets.
	RemoveOld(ctx context.Context, a Address, retentionDays int) error
	// FindByRequestID finds a history record by request ID.
	// It returns the most recent record if multiple matches exist.
	FindByRequestID(ctx context.Context, a Address, requestID string) (string, error)
}

var _ Storage = (*storage)(nil)

type storage struct{}

func New() Storage {
	return &storage{}
}

// FindByRequestID implements Storage.
func (s *storage) FindByRequestID(_ context.Context, a Address, requestID string) (string, error) {
	// Find matching files
	matches, err := filepath.Glob(a.GlobPatternWithRequestID(requestID))
	if err != nil {
		return "", fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	return matches[0], nil
}

// GenerateFilePath implements Storage.
func (s *storage) GenerateFilePath(_ context.Context, a Address, timestamp TimeInUTC, reqID string) string {
	return a.FilePath(timestamp, reqID)
}

// Latest implements Storage.
func (s *storage) Latest(ctx context.Context, a Address, itemLimit int) []string {
	pattern := a.globPattern
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Errorf(ctx, "Failed to find matches for pattern %s: %v", pattern, err)
		return nil
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No matches found for pattern %s", pattern)
		return nil
	}

	return filterLatest(matches, itemLimit, maxWorkers)
}

// LatestAfter implements Storage.
func (s *storage) LatestAfter(_ context.Context, a Address, cutoff TimeInUTC) (string, error) {
	matches, err := filepath.Glob(a.globPattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusData
	}

	ret := filterLatest(matches, 1, maxWorkers)
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

// RemoveOld implements Storage.
func (s *storage) RemoveOld(ctx context.Context, a Address, retentionDays int) error {
	// Find matching files
	matches, err := filepath.Glob(a.globPattern)
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil
	}

	// Calculate the cutoff date
	oldDate := time.Now().AddDate(0, 0, -retentionDays)

	// Process files in parallel
	errs := processFilesParallel(ctx, matches, func(filePath string) error {
		// Check if the file is older than the cutoff date
		info, err := os.Stat(filePath)
		if err != nil {
			logger.Debugf(ctx, "Failed to stat file %s: %v", filePath, err)
			return nil // Skip files we can't stat
		}

		if info.ModTime().Before(oldDate) {
			dir := filepath.Dir(filePath)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", dir, err)
			}
			logger.Debugf(ctx, "Removed old directory %s", dir)
		}
		return nil
	})

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Rename implements Storage.
func (s *storage) Rename(ctx context.Context, o Address, n Address) error {
	// Get the old directory
	if !o.Exists() {
		return nil
	}

	// Create the new directory if it doesn't exist
	if !n.Exists() {
		if err := n.Create(); err != nil {
			return err
		}
	}

	// Find matching files
	matches, err := filepath.Glob(o.globPattern)
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		logger.Debugf(ctx, "No files to rename for %q", o.globPattern)
		return nil
	}

	// Process files in parallel
	errs := processFilesParallel(ctx, matches, func(filePath string) error {
		// Replace the old prefix with the new prefix
		oldDir := filepath.Dir(filePath)
		dirName := filepath.Base(oldDir)
		newName := strings.Replace(dirName, o.prefix, n.prefix, 1)
		newDir := filepath.Join(n.path, newName)

		// Rename the file
		if err := os.Rename(oldDir, newDir); err != nil {
			logger.Error(ctx, "Failed to rename directory", "err", err, "old", oldDir, "new", newDir)
			return fmt.Errorf("failed to rename directory %s to %s: %w", oldDir, newDir, err)
		}

		logger.Debugf(ctx, "Renamed %s to %s", filePath, newDir)
		return nil
	})

	// Try to remove the old directory if it's empty
	if o.IsEmpty() {
		if err := o.Remove(); err != nil {
			logger.Warn(ctx, "Failed to remove old directory", "err", err)
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// processFilesParallel processes files in parallel using a worker pool.
// It returns a slice of errors encountered during processing.
func processFilesParallel(ctx context.Context, files []string, processor func(string) error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, maxWorkers)

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
	t, err := time.Parse(dateTimeFormatUTC, timestampString)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", timestampString, err)
	}
	return t, nil
}
