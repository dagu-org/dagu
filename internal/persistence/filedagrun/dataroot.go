package filedagrun

import (
	// nolint: gosec
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
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

	"github.com/dagu-org/dagu/internal/common/dirlock"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// DataRoot manages the directory structure for run history data.
// It handles the organization of run data in a hierarchical structure
// based on year, month, and day.
type DataRoot struct {
	dirlock.DirLock // Directory lock for concurrent access

	baseDir     string // Base directory for all DAGs
	prefix      string // Sanitized prefix for directory names
	dagRunsDir  string // Path to the dag-runs directory
	globPattern string // Pattern for finding run directories
}

// NewDataRoot creates a new DataRoot instance for managing a DAG's run history.
// It sanitizes the DAG name to create a safe directory structure and applies any provided options.
//
// Parameters:
//   - baseDir: The base directory where all DAG data is stored
//   - dagName: The name of the DAG (can be a path to a YAML file)
//
// Returns:
//   - A configured DataRoot instance
func NewDataRoot(baseDir, dagName string) DataRoot {
	ext := filepath.Ext(dagName)
	root := DataRoot{baseDir: baseDir}

	base := filepath.Base(dagName)
	if fileutil.IsYAMLFile(dagName) {
		// Remove .yaml or .yml extension
		base = strings.TrimSuffix(base, ext)
	}

	// Create a safe directory name from the DAG name
	prefix := fileutil.SafeName(base)
	if prefix != base {
		// If the name was modified for safety, append a hash to ensure uniqueness
		hash := sha256.Sum256([]byte(dagName))
		hashLength := 4 // 4 characters of the hash should be enough
		prefix = prefix + "-" + hex.EncodeToString(hash[:])[0:hashLength]
	}

	root.prefix = prefix
	root.dagRunsDir = filepath.Join(baseDir, root.prefix, "dag-runs")
	root.globPattern = filepath.Join(root.dagRunsDir, "*", "*", "*", DAGRunDirPrefix+"*")
	root.DirLock = dirlock.New(root.dagRunsDir, &dirlock.LockOptions{
		StaleThreshold: 30 * time.Second,      // Default stale threshold
		RetryInterval:  50 * time.Millisecond, // Default retry interval
	})

	return root
}

// NewDataRootWithPrefix creates a new DataRoot instance with a specified prefix.
// This is useful for creating a DataRoot with a specific directory structure
func NewDataRootWithPrefix(baseDir, prefix string) DataRoot {
	dagRunsDir := filepath.Join(baseDir, prefix, "dag-runs")
	return DataRoot{
		baseDir:     baseDir,
		prefix:      prefix,
		dagRunsDir:  dagRunsDir,
		globPattern: filepath.Join(dagRunsDir, "*", "*", "*", DAGRunDirPrefix+"*"),
	}
}

// FindByDAGRunID locates a dag-run by its ID.
// It searches through all dag-run directories to find a match,
// and returns the most recent one if multiple matches are found.
//
// Parameters:
//   - ctx: Context for the operation (unused but kept for interface consistency)
//   - dagRunID: The unique dag-run ID to search for
//
// Returns:
//   - The matching DAGRun instance, or an error if not found
func (dr *DataRoot) FindByDAGRunID(_ context.Context, dagRunID string) (*DAGRun, error) {
	// Find matching files
	matches, err := filepath.Glob(dr.GlobPatternWithDAGRunID(dagRunID))
	if err != nil {
		return nil, fmt.Errorf("failed to make glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", execution.ErrDAGRunIDNotFound, dagRunID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	return NewDAGRun(matches[0])
}

// Latest returns the most recent dag-runs up to the specified limit.
// It searches through the dag-run directories and returns them sorted by timestamp (newest first).
func (dr *DataRoot) Latest(ctx context.Context, itemLimit int) []*DAGRun {
	dagRuns, err := dr.listRecentDAGRuns(ctx, itemLimit)
	if err != nil {
		logger.Error(ctx, "Failed to list recent runs",
			tag.Error(err))
		return nil
	}
	return dagRuns
}

// LatestAfter returns the most recent dag-run that occurred after the specified cutoff time.
// Returns ErrNoStatusData if no dag-run is found or if the latest run is before the cutoff.
func (dr *DataRoot) LatestAfter(ctx context.Context, cutoff execution.TimeInUTC) (*DAGRun, error) {
	runs, err := dr.listRecentDAGRuns(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent runs: %w", err)
	}
	if len(runs) == 0 {
		return nil, execution.ErrNoStatusData
	}
	if runs[0].timestamp.Before(cutoff.Time) {
		return nil, execution.ErrNoStatusData
	}
	return runs[0], nil
}

// CreateDAGRun creates a new dag-run directory with the specified timestamp and ID.
// The directory structure follows the pattern: year/month/day/run-YYYYMMDD_HHMMSS_dagRunID
func (dr *DataRoot) CreateDAGRun(ts execution.TimeInUTC, dagRunID string) (*DAGRun, error) {
	dirName := DAGRunDirPrefix + formatDAGRunTimestamp(ts) + "_" + dagRunID
	dir := filepath.Join(dr.dagRunsDir, ts.Format("2006"), ts.Format("01"), ts.Format("02"), dirName)

	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return NewDAGRun(dir)
}

// GlobPatternWithDAGRunID returns a glob pattern for finding dag-run directories
// that contain the specified dag-run ID in their name.
func (dr DataRoot) GlobPatternWithDAGRunID(dagRunID string) string {
	return filepath.Join(dr.dagRunsDir, "2*", "*", "*", DAGRunDirPrefix+"*"+dagRunID)
}

// Exists checks if the dag-runs directory exists in the file system.
func (dr DataRoot) Exists() bool {
	_, err := os.Stat(dr.dagRunsDir)
	return !os.IsNotExist(err)
}

// Create creates the dag-runs directory if it doesn't already exist.
// Returns nil if the directory already exists or is successfully created.
func (dr DataRoot) Create() error {
	if dr.Exists() {
		return nil
	}
	if err := os.MkdirAll(dr.dagRunsDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dr.dagRunsDir, err)
	}
	return nil
}

// IsEmpty checks if the dag-runs directory exists and contains no dag-run directories.
// Returns true if the directory doesn't exist or contains no dag-runs.
func (dr DataRoot) IsEmpty() bool {
	_, err := os.Stat(dr.dagRunsDir)
	if err != nil && os.IsNotExist(err) {
		return true
	}
	matches, err := filepath.Glob(dr.globPattern)
	if err != nil {
		return false
	}
	if len(matches) == 0 {
		return true
	}
	return false
}

// Remove completely removes the dag-runs directory and all its contents.
// This operation cannot be undone.
func (dr DataRoot) Remove() error {
	if err := os.RemoveAll(dr.dagRunsDir); err != nil {
		return fmt.Errorf("failed to remove directory %s: %w", dr.dagRunsDir, err)
	}
	return nil
}

// Rename moves all dag-run directories from this DataRoot to a new DataRoot location.
// This operation preserves the hierarchical structure and removes empty directories.
// Both DataRoots must share the same base directory.
func (dr DataRoot) Rename(ctx context.Context, newRoot DataRoot) error {
	if !dr.Exists() {
		return nil
	}
	if dr.baseDir != newRoot.baseDir {
		return fmt.Errorf("cannot rename to a different base directory: %s -> %s", dr.baseDir, newRoot.baseDir)
	}
	if !newRoot.Exists() {
		if err := newRoot.Create(); err != nil {
			return err
		}
	}

	matches, err := filepath.Glob(dr.globPattern)
	if err != nil {
		return fmt.Errorf("failed to glob pattern: %w", err)
	}

	// Process files in parallel
	errs := processFilesParallel(matches, func(targetDir string) error {
		// Construct the new directory path
		day := filepath.Base(filepath.Dir(targetDir))
		month := filepath.Base(filepath.Dir(filepath.Dir(targetDir)))
		year := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(targetDir))))
		newDir := filepath.Join(newRoot.dagRunsDir, year, month, day, filepath.Base(targetDir))

		// Enrich context with directory information for error logging
		dirCtx := logger.WithValues(ctx,
			slog.String("oldDir", targetDir),
			slog.String("newDir", newDir))

		// Make sure the new directory exists
		if err := os.MkdirAll(filepath.Dir(newDir), 0750); err != nil {
			logger.Error(dirCtx, "Failed to create new directory",
				tag.Error(err))
			return fmt.Errorf("failed to create directory %s: %w", newDir, err)
		}

		// Rename the file
		if err := os.Rename(targetDir, newDir); err != nil {
			logger.Error(dirCtx, "Failed to rename directory",
				tag.Error(err))
			return fmt.Errorf("failed to rename %s to %s: %w", targetDir, newDir, err)
		}

		dr.removeEmptyDir(ctx, filepath.Dir(targetDir))

		return nil
	})

	if len(errs) > 0 {
		return fmt.Errorf("failed to rename files: %w", errors.Join(errs...))
	}

	if dr.IsEmpty() {
		if err := dr.Remove(); err != nil {
			return err
		}
	}
	return nil
}

// RemoveOld removes old dag-runs older than the specified retention days.
// It only removes records older than the specified retention days.
// If retentionDays is negative, no files will be removed.
// If retentionDays is zero, all files will be removed.
// If retentionDays is positive, only files older than the specified number of days will be removed.
// It also removes empty directories in the hierarchy.
func (dr DataRoot) RemoveOld(ctx context.Context, retentionDays int) error {
	keepTime := execution.NewUTC(time.Now().AddDate(0, 0, -retentionDays))
	dagRuns := dr.listDAGRunsInRange(ctx, execution.TimeInUTC{}, keepTime, &listDAGRunsInRangeOpts{})

	for _, r := range dagRuns {
		// Enrich context with run directory for all subsequent logs in this iteration
		runCtx := logger.WithValues(ctx, tag.Dir(r.baseDir))

		latestAttempt, err := r.LatestAttempt(ctx, nil)
		if err != nil {
			logger.Error(runCtx, "Failed to get latest attempt",
				tag.Error(err))
			continue
		}
		lastUpdate, err := latestAttempt.ModTime()
		if err != nil {
			logger.Error(runCtx, "Failed to get last modified time",
				tag.Error(err))
			continue
		}
		latestStatus, err := latestAttempt.ReadStatus(ctx)
		if err != nil {
			logger.Error(runCtx, "Failed to read status",
				tag.Error(err))
			continue
		}
		if latestStatus.Status.IsActive() {
			// If the run is still active, skip it
			logger.Debug(runCtx, "Skipping active run",
				tag.Status(latestStatus.Status.String()))
			continue
		}
		if lastUpdate.After(keepTime.Time) {
			continue
		}
		if err := r.Remove(ctx); err != nil {
			logger.Error(runCtx, "Failed to remove run",
				tag.Error(err))
		}
		dr.removeEmptyDir(ctx, filepath.Dir(r.baseDir))
	}
	return nil
}

func (dr DataRoot) removeEmptyDir(ctx context.Context, dayDir string) {
	monthDir := filepath.Dir(dayDir)
	yearDir := filepath.Dir(monthDir)

	// Helper function to remove directory with context-enriched logging
	removeDir := func(dirPath, dirType string) {
		dirCtx := logger.WithValues(ctx, tag.Dir(dirPath))
		if isDirEmpty(dirPath) {
			if err := os.Remove(dirPath); err != nil {
				logger.Error(dirCtx, fmt.Sprintf("Failed to remove %s directory", dirType),
					tag.Error(err))
			}
		}
	}

	removeDir(dayDir, "day")
	removeDir(monthDir, "month")
	removeDir(yearDir, "year")
}

// listDAGRunsInRangeOpts contains options for listing dag-runs in a range
type listDAGRunsInRangeOpts struct {
	limit int
}

func (dr DataRoot) listDAGRunsInRange(ctx context.Context, start, end execution.TimeInUTC, opts *listDAGRunsInRangeOpts) []*DAGRun {
	var result []*DAGRun
	var lock sync.Mutex

	// If start time is after end time, return empty result
	if !start.IsZero() && !end.IsZero() && start.After(end.Time) {
		return nil
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
		endDate = time.Now().UTC()
	} else {
		endDate = end.Time
	}

	years, err := listDirsSorted(dr.dagRunsDir, false, reYear)
	if err != nil {
		return nil
	}

	for _, year := range years {
		yearInt, _ := strconv.Atoi(year)
		yearPath := filepath.Join(dr.dagRunsDir, year)

		// Skip years outside the range
		if yearInt < startDate.Year() || yearInt > endDate.Year() {
			continue
		}

		// Get all months in the year
		months, err := listDirsSorted(yearPath, false, reMonth)
		if err != nil {
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
				continue
			}

			for _, day := range days {
				dayInt, _ := strconv.Atoi(day)
				dayPath := filepath.Join(monthPath, day)

				// Skip days outside the range
				if (yearInt == startDate.Year() && monthInt == int(startDate.Month()) && dayInt < startDate.Day()) ||
					(yearInt == endDate.Year() && monthInt == int(endDate.Month()) && dayInt > endDate.Day()) {
					continue
				}

				// Find all status files for this day
				files, err := filepath.Glob(filepath.Join(dayPath, DAGRunDirPrefix+"*"))
				if err != nil {
					continue
				}

				_ = processFilesParallel(files, func(filePath string) error {
					run, err := NewDAGRun(filePath)
					if err != nil {
						logger.Debug(ctx, "Failed to create run from file",
							tag.File(filePath),
							tag.Error(err))
						return err
					}
					// Check if the timestamp is within the range
					if (start.IsZero() || !run.timestamp.Before(startDate)) &&
						(end.IsZero() || run.timestamp.Before(endDate)) {
						lock.Lock()
						result = append(result, run)
						lock.Unlock()
					}
					return nil
				})

				if opts != nil && opts.limit > 0 && len(result) >= opts.limit {
					// Limit reached, break out of the loop
					goto BREAK
				}
			}
		}
	}

BREAK:

	sort.Slice(result, func(i, j int) bool {
		return result[i].timestamp.After(result[j].timestamp)
	})

	return result
}

func (dr DataRoot) listRecentDAGRuns(_ context.Context, itemLimit int) ([]*DAGRun, error) {
	var founds []string

	years, err := listDirsSorted(dr.dagRunsDir, true, reYear)
	if err != nil {
		return nil, fmt.Errorf("failed to list years: %w", err)
	}

YEAR_LOOP:
	for _, year := range years {
		yearPath := filepath.Join(dr.dagRunsDir, year)
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
				runs, err := filepath.Glob(filepath.Join(dayPath, DAGRunDirPrefix+"*"))
				if err != nil {
					return nil, fmt.Errorf("failed to find matches for pattern %s: %w", dayPath, err)
				}
				founds = append(founds, runs...)
				if len(founds) >= itemLimit {
					break YEAR_LOOP
				}
			}
		}
	}

	sort.Strings(founds)
	slices.Reverse(founds)
	if len(founds) > itemLimit {
		founds = founds[:itemLimit]
	}

	var result []*DAGRun
	for _, f := range founds {
		run, err := NewDAGRun(f)
		if err != nil {
			continue
		}
		result = append(result, run)
	}

	return result, nil
}

// listDirsSorted lists directories in the given path, optionally in reverse order.
// It can filter directories based on a regular expression pattern and sort them
// either in ascending or descending order.
//
// Parameters:
//   - path: Directory path to list
//   - reverse: If true, sort in descending order; if false, sort in ascending order
//   - pattern: Optional regex pattern to filter directory names (nil means no filtering)
//
// Returns:
//   - A sorted slice of directory names, or nil if the directory doesn't exist
//   - An error if the directory couldn't be read
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
			name := entry.Name()
			if entry.IsDir() && pattern.MatchString(name) {
				dirs = append(dirs, name)
			}
		}
	} else {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				dirs = append(dirs, name)
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

// processFilesParallel processes files in parallel using a worker pool.
// It limits concurrency to the number of available CPU cores and handles
// context cancellation gracefully.
//
// Parameters:
//   - ctx: Context for the operation, which can be used to cancel processing
//   - files: Slice of file paths to process
//   - processor: Function to apply to each file path
//
// Returns:
//   - A slice of errors encountered during processing
func processFilesParallel(files []string, processor func(string) error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, runtime.NumCPU())

	for _, file := range files {
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

// isDirEmpty checks if a directory is empty.
// It returns true if the directory exists and contains no entries,
// and false if the directory doesn't exist or contains entries.
func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}

// Regular expressions for directory structure validation
var (
	reYear  = regexp.MustCompile(`^\d{4}$`) // Matches 4-digit year directories (e.g., "2023")
	reMonth = regexp.MustCompile(`^\d{2}$`) // Matches 2-digit month directories (e.g., "01" for January)
	reDay   = regexp.MustCompile(`^\d{2}$`) // Matches 2-digit day directories (e.g., "15" for the 15th day)
)
