package jsondb

import (
	// nolint: gosec
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
)

type DataRoot struct {
	baseDir       string
	dagName       string
	prefix        string
	executionsDir string
	globPattern   string
	rootDAG       *digraph.RootDAG
}

type RootOption func(*DataRoot)

func WithRootDAG(rootDAG *digraph.RootDAG) RootOption {
	return func(dr *DataRoot) {
		dr.rootDAG = rootDAG
	}
}

func NewDataRoot(baseDir, dagName string, opts ...RootOption) DataRoot {
	ext := filepath.Ext(dagName)
	root := DataRoot{baseDir: baseDir, dagName: dagName}

	for _, opt := range opts {
		opt(&root)
	}

	base := filepath.Base(dagName)
	if fileutil.IsYAMLFile(dagName) {
		// Remove .yaml or .yml extension
		base = strings.TrimSuffix(base, ext)
	}

	prefix := fileutil.SafeName(base)
	if prefix != base {
		hash := sha256.Sum256([]byte(dagName))
		hashLength := 4 // 4 characters of the hash should be enough
		prefix = prefix + "-" + hex.EncodeToString(hash[:])[0:hashLength]
	}

	root.prefix = prefix
	root.executionsDir = filepath.Join(baseDir, root.prefix, "executions")
	root.globPattern = filepath.Join(root.executionsDir, "*", "*", "*", "exec_*")

	return root
}

func (dr *DataRoot) FindByRequestID(_ context.Context, requestID string) (*Execution, error) {
	// Find matching files
	matches, err := filepath.Glob(dr.GlobPatternWithRequestID(requestID))
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", persistence.ErrRequestIDNotFound, requestID)
	}

	// Sort matches by timestamp (most recent first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	return NewExecution(matches[0])
}

func (dr *DataRoot) Latest(ctx context.Context, itemLimit int) []*Execution {
	executions, err := dr.listRecentExecutions(ctx, itemLimit)
	if err != nil {
		logger.Errorf(ctx, "failed to list recent executions: %v", err)
		return nil
	}
	return executions
}

func (dr *DataRoot) LatestAfter(ctx context.Context, cutoff TimeInUTC) (*Execution, error) {
	executions, err := dr.listRecentExecutions(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent executions: %w", err)
	}
	if len(executions) == 0 {
		return nil, persistence.ErrNoStatusData
	}
	if executions[0].timestamp.Before(cutoff.Time) {
		return nil, persistence.ErrNoStatusData
	}
	return executions[0], nil
}

func (dr *DataRoot) ListInRange(ctx context.Context, start, end TimeInUTC) []*Execution {
	return dr.listInRange(ctx, start, end)
}

func (dr *DataRoot) CreateExecution(timestamp TimeInUTC, reqID string) (*Execution, error) {
	dirName := "exec_" + timestamp.Format(dateTimeFormatUTC) + "_" + reqID
	dir := filepath.Join(dr.executionsDir, timestamp.Format("2006"), timestamp.Format("01"), timestamp.Format("02"), dirName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("%w: %s : %s", storage.ErrCreateNewDirectory, dir, err)
	}

	return NewExecution(dir)
}

func (dr DataRoot) GlobPatternWithRequestID(requestID string) string {
	return filepath.Join(dr.executionsDir, "2*", "*", "*", "exec_*"+requestID+"*")
}

func (dr DataRoot) FilePath(timestamp storage.TimeInUTC, requestID string) string {
	year := timestamp.Format("2006")
	month := timestamp.Format("01")
	date := timestamp.Format("02")
	ts := timestamp.Format(dateTimeFormatUTC)
	dirName := "exec_" + ts + "_" + requestID
	return filepath.Join(dr.executionsDir, year, month, date, dirName, "status"+dataFileExtension)
}

func (dr DataRoot) Exists() bool {
	_, err := os.Stat(dr.executionsDir)
	return !os.IsNotExist(err)
}

func (dr DataRoot) Create() error {
	if dr.Exists() {
		return nil
	}
	if err := os.MkdirAll(dr.executionsDir, 0755); err != nil {
		return fmt.Errorf("%w: %s : %s", storage.ErrCreateNewDirectory, dr.executionsDir, err)
	}
	return nil
}

func (dr DataRoot) IsEmpty() bool {
	_, err := os.Stat(dr.executionsDir)
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

func (dr DataRoot) Remove() error {
	if err := os.RemoveAll(dr.executionsDir); err != nil {
		return fmt.Errorf("%w: %s : %s", storage.ErrRemoveDirectory, dr.executionsDir, err)
	}
	return nil
}

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
	errs := processFilesParallel(ctx, matches, func(targetDir string) error {
		// Construct the new directory path
		day := filepath.Base(filepath.Dir(targetDir))
		month := filepath.Base(filepath.Dir(filepath.Dir(targetDir)))
		year := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(targetDir))))
		newDir := filepath.Join(newRoot.executionsDir, year, month, day, filepath.Base(targetDir))

		// Make sure the new directory exists
		if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
			logger.Error(ctx, "Failed to create new directory", "err", err, "new", newDir)
			return fmt.Errorf("failed to create directory %s: %w", newDir, err)
		}

		// Rename the file
		if err := os.Rename(targetDir, newDir); err != nil {
			logger.Error(ctx, "Failed to rename directory", "err", err, "old", targetDir, "new", newDir)
			return fmt.Errorf("failed to rename %s to %s: %w", targetDir, newDir, err)
		}

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

func (dr DataRoot) RemoveOld(ctx context.Context, retentionDays int) error {
	keepTime := NewUTC(time.Now().AddDate(0, 0, -retentionDays))
	executions := dr.listInRange(ctx, TimeInUTC{}, keepTime)
	for _, exec := range executions {
		lastUpdate, err := exec.LastUpdated(ctx)
		if err != nil {
			logger.Errorf(ctx, "failed to get last update time for %s: %v", exec.baseDir, err)
			continue
		}
		if lastUpdate.After(keepTime.Time) {
			continue
		}
		if err := exec.Remove(); err != nil {
			logger.Errorf(ctx, "failed to remove execution %s: %v", exec.baseDir, err)
		}
	}
	return nil
}

func (dr DataRoot) listInRange(ctx context.Context, start, end TimeInUTC) []*Execution {
	var result []*Execution
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

	years, err := listDirsSorted(dr.executionsDir, false, reYear)
	if err != nil {
		return nil
	}

	for _, year := range years {
		yearInt, _ := strconv.Atoi(year)
		yearPath := filepath.Join(dr.executionsDir, year)

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
				files, err := filepath.Glob(filepath.Join(dayPath, "exec_*"))
				if err != nil {
					continue
				}

				_ = processFilesParallel(ctx, files, func(filePath string) error {
					exec, err := NewExecution(filePath)
					if err != nil {
						logger.Debugf(ctx, "Failed to create execution from file %s: %v", filePath)
						return err
					}
					// Check if the timestamp is within the range
					if (start.IsZero() || !exec.timestamp.Before(startDate)) &&
						(end.IsZero() || exec.timestamp.Before(endDate)) {
						lock.Lock()
						result = append(result, exec)
						lock.Unlock()
					}
					return nil
				})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].timestamp.After(result[j].timestamp)
	})

	return result
}

func (dr DataRoot) listRecentExecutions(_ context.Context, itemLimit int) ([]*Execution, error) {
	var founds []string

	years, err := listDirsSorted(dr.executionsDir, true, reYear)
	if err != nil {
		return nil, fmt.Errorf("failed to list years: %w", err)
	}

YEAR_LOOP:
	for _, year := range years {
		yearPath := filepath.Join(dr.executionsDir, year)
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
				executions, err := filepath.Glob(filepath.Join(dayPath, "exec_*"))
				if err != nil {
					return nil, fmt.Errorf("failed to find matches for pattern %s: %w", dayPath, err)
				}
				founds = append(founds, executions...)
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

	var result []*Execution
	for _, f := range founds {
		exec, err := NewExecution(f)
		if err != nil {
			continue
		}
		result = append(result, exec)
	}

	return result, nil
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

// processFilesParallel processes files in parallel using a worker pool.
// It returns a slice of errors encountered during processing.
func processFilesParallel(ctx context.Context, files []string, processor func(string) error) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, runtime.NumCPU())

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

var (
	reYear  = regexp.MustCompile(`^\d{4}$`)
	reMonth = regexp.MustCompile(`^\d{2}$`)
	reDay   = regexp.MustCompile(`^\d{2}$`)
)

// dateTimeFormatUTC is the format for execution timestamps.
var dateTimeFormatUTC = "20060102_150405_000Z"

// dataFileExtension is the file extension for history record files.
var dataFileExtension = ".dat"
