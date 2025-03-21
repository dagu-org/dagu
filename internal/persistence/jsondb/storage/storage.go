package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
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
	recents, err := s.listRecent(ctx, a, itemLimit)
	if err != nil {
		logger.Errorf(ctx, "Failed to list recent files: %v", err)
		return nil
	}

	return recents
}

// LatestAfter implements Storage.
func (s *storage) LatestAfter(ctx context.Context, a Address, cutoff TimeInUTC) (string, error) {
	files := s.listInRange(ctx, a, cutoff, TimeInUTC{time.Now().UTC()})
	ret := filterLatest(files, 1, maxWorkers)
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

var (
	reYear  = regexp.MustCompile(`^\d{4}$`)
	reMonth = regexp.MustCompile(`^\d{2}$`)
	reDay   = regexp.MustCompile(`^\d{2}$`)
)

func (s *storage) listRecent(_ context.Context, a Address, n int) ([]string, error) {
	var result []string
	years, err := listDirsSorted(a.path, true, reYear)
	if err != nil {
		return nil, fmt.Errorf("failed to list years: %w", err)
	}

OUT:
	for _, year := range years {
		yearPath := filepath.Join(a.path, year)
		months, err := listDirsSorted(yearPath, true, reMonth)
		if err != nil {
			return nil, fmt.Errorf("failed to list months: %w", err)
		}

		for _, month := range months {
			monthPath := filepath.Join(yearPath, month)
			days, err := listDirsSorted(monthPath, true, reDay)
			if err != nil {
				return nil, fmt.Errorf("failed to list days: %w", err)
			}

			for _, day := range days {
				dayPath := filepath.Join(monthPath, day)
				files, err := filepath.Glob(filepath.Join(dayPath, "2*", "status.dat"))
				if err != nil {
					return nil, fmt.Errorf("failed to find matches for pattern %s: %w", dayPath, err)
				}

				result = append(result, files...)
				if len(result) >= n {
					break OUT
				}
			}
		}
	}

	sort.Strings(result)
	slices.Reverse(result)
	if len(result) > n {
		return result[:n], nil
	}

	return result, nil
}

// listStatusInRange retrieves all status files for a specific date range.
// The range is inclusive of the start time and exclusive of the end time.
func (s *storage) listInRange(ctx context.Context, a Address, start, end TimeInUTC) []string {
	var result []string
	var lock sync.Mutex

	// If start time is after end time, return empty result
	if !start.IsZero() && !end.IsZero() && start.After(end.Time) {
		return result
	}

	// Calculate the date range to search
	var startDate, endDate time.Time
	if start.IsZero() {
		// If start is zero, use a very old date to include all files
		startDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		startDate = start.Time
	}

	if end.IsZero() {
		// If end is zero, use current time
		endDate = time.Now().UTC()
	} else {
		endDate = end.Time
	}

	// Get all years in the range
	years, err := listDirsSorted(a.path, true, reYear)
	if err != nil {
		logger.Errorf(ctx, "Failed to list years: %v", err)
		return nil
	}

	for _, year := range years {
		yearInt, _ := strconv.Atoi(year)
		yearPath := filepath.Join(a.path, year)

		// Skip years outside the range
		if yearInt < startDate.Year() || yearInt > endDate.Year() {
			continue
		}

		// Get all months in the year
		months, err := listDirsSorted(yearPath, true, reMonth)
		if err != nil {
			logger.Errorf(ctx, "Failed to list months in %s: %v", year, err)
			continue
		}

		for _, month := range months {
			monthInt, _ := strconv.Atoi(month)
			monthPath := filepath.Join(yearPath, month)

			// Skip months outside the range
			if (yearInt == startDate.Year() && monthInt < int(startDate.Month())) ||
				(yearInt == endDate.Year() && monthInt > int(endDate.Month())) {
				continue
			}

			// Get all days in the month
			days, err := listDirsSorted(monthPath, true, reDay)
			if err != nil {
				logger.Errorf(ctx, "Failed to list days in %s/%s: %v", year, month, err)
				continue
			}

			for _, day := range days {
				dayPath := filepath.Join(monthPath, day)

				// Find all status files for this day
				files, err := filepath.Glob(filepath.Join(dayPath, "2*", "status"+dataFileExtension))
				if err != nil {
					logger.Errorf(ctx, "Failed to glob pattern in %s/%s/%s: %v", year, month, day, err)
					continue
				}

				_ = processFilesParallel(ctx, files, func(filePath string) error {
					timestamp, err := parseFileTimestamp(filePath)
					if err != nil {
						logger.Debugf(ctx, "Failed to parse timestamp from file %s: %v", filePath, err)
						return err
					}
					// Check if the timestamp is within the range
					if (start.IsZero() || !timestamp.Before(startDate)) &&
						(end.IsZero() || timestamp.Before(endDate)) {
						lock.Lock()
						result = append(result, filePath)
						lock.Unlock()
					}
					return nil
				})
			}
		}
	}

	return result
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

		// Construct the new directory path
		day := filepath.Base(filepath.Dir(oldDir))
		month := filepath.Base(filepath.Dir(filepath.Dir(oldDir)))
		year := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(oldDir))))
		newDir := filepath.Join(n.path, year, month, day, newName)

		// Make sure the new directory exists
		if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
			logger.Error(ctx, "Failed to create new directory", "err", err, "new", newDir)
			return fmt.Errorf("failed to create directory %s: %w", newDir, err)
		}

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

// listDirsSorted lists directories in the given path, optionally in reverse order.
func listDirsSorted(path string, reverse bool, pattern *regexp.Regexp) ([]string, error) {
	entries, err := os.ReadDir(path)
	// If the directory does not exist, return nil
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var dirs []string
	if pattern != nil {
		for _, entry := range entries {
			if entry.IsDir() && pattern.MatchString(entry.Name()) {
				dirs = append(dirs, entry.Name())
			}
		}
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}
	}

	if reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	} else {
		sort.Strings(dirs)
	}

	return dirs, nil
}
