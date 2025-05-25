package localproc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// ProcGroup is a struct that manages process files for a given workflow name.
type ProcGroup struct {
	name      string
	baseDir   string
	staleTime time.Duration
	mu        sync.Mutex
}

// procFilePrefix is the prefix for the proc files
const procFilePrefix = "proc_"

// procFileRegex is a regex pattern to match the proc file name format
var procFileRegex = regexp.MustCompile(`^proc_\d{8}_\d{6}Z_.*\.proc$`)

// NewProcGroup creates a new instance of a ProcGroup with the specified base directory and workflow name.
func NewProcGroup(baseDir, name string, staleTime time.Duration) *ProcGroup {
	return &ProcGroup{
		baseDir:   baseDir,
		name:      name,
		staleTime: staleTime,
	}
}

// Count retrieves the count of alive proc files for the specified workflow name.
func (pg *ProcGroup) Count(ctx context.Context, name string) (int, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// If directory does not exist, return 0
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}

	// Grep for all proc files in the directory
	files, err := filepath.Glob(filepath.Join(pg.baseDir, procFilePrefix+"*.proc"))
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
			logger.Error(ctx, "failed to remove stale file %s: %v", file, err)
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
	data, err := os.ReadFile(file)
	if err != nil {
		logger.Warn(ctx, "failed to read file %s: %v", file, err)
		return false
	}

	var unixTime int64

	// It is assumed that the first 8 bytes of the file contain a timestamp in nanoseconds (unix time).
	if len(data) < 8 {
		logger.Warn(ctx, "file %s is too short, expected at least 8 bytes", file)
		return false
	}

	// Parse the timestamp from the file
	unixTime = int64(binary.BigEndian.Uint64(data[:8]))
	parsedTime := time.Unix
	duration := time.Since(time.Unix(unixTime, 0))
	if duration < pg.staleTime {
		logger.Debug(ctx, "proc file %s is not stale, last modified at %s", file, parsedTime)
		return false
	}
	logger.Debug(ctx, "proc file %s is stale, last modified at %s", file, parsedTime)
	return true
}

// GetProc retrieves a proc file for the specified workflow reference.
// It returns a new Proc instance with the generated file name.
func (pg *ProcGroup) Acquire(ctx context.Context, workflow digraph.DAGRunRef) (*ProcHandle, error) {
	// Sanity check the workflow reference
	if pg.name != workflow.Name {
		return nil, fmt.Errorf("workflow name %s does not match proc file name %s", workflow.Name, pg.name)
	}
	// Generate the proc file name
	fileName := pg.getFileName(models.NewUTC(time.Now()), workflow)
	return NewProcHandler(fileName, models.ProcMeta{
		StartedAt: time.Now().Unix(),
	}), nil
}

// getFileName generates a proc file name based on the workflow reference.
func (pg *ProcGroup) getFileName(t models.TimeInUTC, workflow digraph.DAGRunRef) string {
	timestamp := t.Format(dateTimeFormatUTC)
	fileName := procFilePrefix + timestamp + "Z_" + workflow.ID + ".proc"
	return filepath.Join(pg.baseDir, fileName)
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"
