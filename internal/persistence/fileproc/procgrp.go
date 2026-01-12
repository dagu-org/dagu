package fileproc

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/dirlock"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// ProcGroup is a struct that manages process files for a given DAG name.
type ProcGroup struct {
	dirlock.DirLock

	groupName string
	baseDir   string
	staleTime time.Duration
	mu        sync.Mutex
}

// procFilePrefix is the prefix for the proc files
const procFilePrefix = "proc_"

// procFileRegex is a regex pattern to match the proc file name format
var procFileRegex = regexp.MustCompile(`^proc_\d{8}_\d{6}Z_.*\.proc$`)

// NewProcGroup creates a new instance of a ProcGroup with the specified base directory and DAG name.
func NewProcGroup(baseDir, groupName string, staleTime time.Duration) *ProcGroup {
	dirLock := dirlock.New(baseDir, &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,
		RetryInterval:  100 * time.Millisecond,
	})
	return &ProcGroup{
		DirLock:   dirLock,
		baseDir:   baseDir,
		groupName: groupName,
		staleTime: staleTime,
	}
}

func (pg *ProcGroup) CountByDAGName(ctx context.Context, dagName string) (int, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// If directory does not exist, return 0
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}

	// Grep for all proc files in subdirectories
	files, err := filepath.Glob(filepath.Join(pg.baseDir, dagName, procFilePrefix+"*.proc"))
	if err != nil {
		return 0, err
	}

	aliveCount := 0
	for _, file := range files {
		if !procFileRegex.MatchString(filepath.Base(file)) {
			continue
		}
		// Check if the file is stale
		if !pg.isStale(ctx, file) {
			aliveCount++
			continue
		}
		// File is stale, remove it
		if err := os.Remove(file); err != nil {
			logger.Error(ctx, "Failed to remove stale file",
				tag.File(file),
				tag.Error(err))
		}
		continue
	}

	return aliveCount, nil
}

// Count retrieves the count of alive proc files for the specified DAG name.
func (pg *ProcGroup) Count(ctx context.Context) (int, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// If directory does not exist, return 0
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}

	// Grep for all proc files in subdirectories
	files, err := filepath.Glob(filepath.Join(pg.baseDir, "*", procFilePrefix+"*.proc"))
	if err != nil {
		return 0, err
	}

	aliveCount := 0
	for _, file := range files {
		if !procFileRegex.MatchString(filepath.Base(file)) {
			continue
		}
		// Check if the file is stale
		if !pg.isStale(ctx, file) {
			aliveCount++
			continue
		}
		// File is stale, remove it
		if err := os.Remove(file); err != nil {
			logger.Error(ctx, "Failed to remove stale file",
				tag.File(file),
				tag.Error(err))
		}
		continue
	}

	return aliveCount, nil
}

// isStale checks if the proc file is stale based on its content (timestamp).
func (pg *ProcGroup) isStale(ctx context.Context, file string) bool {
	// Check if the file exists
	fileInfo, err := os.Stat(file)
	if err != nil {
		return true // File does not exist, consider it stale
	}

	if time.Since(fileInfo.ModTime()) < pg.staleTime {
		return false // File is not stale
	}

	// Check if the file is stale by checking its content (timestamp).
	data, err := os.ReadFile(file) // nolint:gosec
	if err != nil {
		logger.Warn(ctx, "Failed to read file",
			tag.File(file),
			tag.Error(err))
		// If we can't read the file, consider it stale
		return true
	}

	// It is assumed that the first 8 bytes of the file contain a timestamp in seconds (unix time).
	if len(data) < 8 {
		logger.Warn(ctx, "File is too short, considering it stale",
			tag.File(file),
			tag.Size(len(data)))
		return true
	}

	// Parse the timestamp from the file
	unixTime := int64(binary.BigEndian.Uint64(data[:8])) // nolint:gosec

	// Validate the timestamp is reasonable (not in the future, not too old)
	now := time.Now()
	parsedTime := time.Unix(unixTime, 0)

	if parsedTime.After(now.Add(5 * time.Minute)) {
		logger.Warn(ctx, "Proc file has timestamp in the future, considering it stale",
			tag.File(file),
			tag.Timestamp(parsedTime))
		return true
	}

	duration := now.Sub(parsedTime)
	if duration < pg.staleTime {
		logger.Debug(ctx, "Proc file is not stale",
			tag.File(file),
			tag.Timestamp(parsedTime),
			tag.Duration(duration))
		return false
	}
	logger.Debug(ctx, "Proc file is stale",
		tag.File(file),
		tag.Timestamp(parsedTime),
		tag.Duration(duration))
	return true
}

// GetProc retrieves a proc file for the specified dag-run reference.
// It returns a new Proc instance with the generated file name.
func (pg *ProcGroup) Acquire(_ context.Context, dagRun exec.DAGRunRef) (*ProcHandle, error) {
	// Generate the proc file name
	fileName := pg.getFileName(exec.NewUTC(time.Now()), dagRun)
	return NewProcHandler(fileName, exec.ProcMeta{
		StartedAt: time.Now().Unix(),
	}), nil
}

// getFileName generates a proc file name based on the dag-run reference and the current time.
func (pg *ProcGroup) getFileName(t exec.TimeInUTC, dagRun exec.DAGRunRef) string {
	timestamp := t.Format(dateTimeFormatUTC)
	fileName := procFilePrefix + timestamp + "Z_" + dagRun.ID + ".proc"
	return filepath.Join(pg.baseDir, dagRun.Name, fileName)
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"

// IsRunAlive checks if a specific DAG run has an alive process file.
func (pg *ProcGroup) IsRunAlive(ctx context.Context, dagRun exec.DAGRunRef) (bool, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// If directory does not exist, return false
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	// Look for proc files with the specific run ID
	pattern := filepath.Join(pg.baseDir, dagRun.Name, procFilePrefix+"*_"+dagRun.ID+".proc")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return false, err
	}

	// Check each matching file
	for _, file := range files {
		if !procFileRegex.MatchString(filepath.Base(file)) {
			continue
		}
		// Check if the file is stale
		if !pg.isStale(ctx, file) {
			return true, nil
		}
		// File is stale, remove it
		if err := os.Remove(file); err != nil {
			logger.Error(ctx, "Failed to remove stale file",
				tag.File(file),
				tag.Error(err))
		}
		// Remove parent directory if it's empty
		_ = os.Remove(filepath.Dir(file))
	}

	return false, nil
}

// ListAlive returns a list of alive DAG runs by scanning process files.
func (pg *ProcGroup) ListAlive(ctx context.Context) ([]exec.DAGRunRef, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// If directory does not exist, return empty list
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return []exec.DAGRunRef{}, nil
	}

	// Grep for all proc files in the directory
	files, err := filepath.Glob(filepath.Join(pg.baseDir, "*", procFilePrefix+"*.proc"))
	if err != nil {
		return nil, err
	}

	var aliveRuns []exec.DAGRunRef
	for _, file := range files {
		basename := filepath.Base(file)
		if !procFileRegex.MatchString(basename) {
			continue
		}
		// Check if the file is stale
		if !pg.isStale(ctx, file) {
			// Extract the run ID from the filename
			// Format: proc_YYYYMMDD_HHMMSSZ_<runID>.proc
			runID := extractRunIDFromFileName(basename)
			dagName := filepath.Base(filepath.Dir(file))
			if runID != "" {
				aliveRuns = append(aliveRuns, exec.DAGRunRef{
					Name: dagName,
					ID:   runID,
				})
			}
			continue
		}
		// File is stale, remove it
		if err := os.Remove(file); err != nil {
			logger.Error(ctx, "Failed to remove stale file",
				tag.File(file),
				tag.Error(err))
		}
	}

	return aliveRuns, nil
}

// extractRunIDFromFileName extracts the run ID from a proc file name.
// Format: proc_YYYYMMDD_HHMMSSZ_<runID>.proc
func extractRunIDFromFileName(filename string) string {
	// Remove the prefix and suffix
	if !procFileRegex.MatchString(filename) {
		return ""
	}
	// Remove "proc_" prefix and ".proc" suffix
	trimmed := filename[len(procFilePrefix):]
	trimmed = trimmed[:len(trimmed)-5] // Remove ".proc"

	// Find the second underscore (after the date) to extract run ID
	// Format after removing prefix: YYYYMMDD_HHMMSSZ_<runID>
	firstUnderscore := -1
	secondUnderscore := -1
	for i, r := range trimmed {
		if r == '_' {
			if firstUnderscore == -1 {
				firstUnderscore = i
			} else if secondUnderscore == -1 {
				secondUnderscore = i
				break
			}
		}
	}

	if secondUnderscore != -1 && secondUnderscore < len(trimmed)-1 {
		return trimmed[secondUnderscore+1:]
	}

	return ""
}
