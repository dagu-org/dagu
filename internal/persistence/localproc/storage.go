package localproc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// procFilePrefix is the prefix for the proc files
const procFilePrefix = "proc_"

// procFileRegex is a regex pattern to match the proc file name format
var procFileRegex = regexp.MustCompile(`^proc_\d{8}_\d{6}Z_.*\.proc$`)

var _ models.ProcStorage = (*Storage)(nil)

// Storage is a struct that implements the ProcStorage interface.
type Storage struct {
	baseDir   string
	staleTime time.Duration
}

// New creates a new instance of Storage with the specified base directory.
func New(baseDir string) *Storage {
	return &Storage{
		baseDir:   baseDir,
		staleTime: time.Second * 45,
	}
}

// Count implements models.ProcStorage.
func (s *Storage) Count(ctx context.Context, name string) (int, error) {
	dir := filepath.Join(s.baseDir, name)

	// Grep for all proc files in the directory
	files, err := filepath.Glob(filepath.Join(dir, procFilePrefix+"*.proc"))
	if err != nil {
		return 0, err
	}

	aliveCount := 0
	for _, file := range files {
		if !procFileRegex.MatchString(filepath.Base(file)) {
			continue
		}
		// Check if the file is stale
		fileInfo, err := os.Stat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			logger.Error(ctx, "failed to stat file %s: %v", file, err)
			continue
		}
		if time.Since(fileInfo.ModTime()) > s.staleTime {
			// File is stale, remove it
			if err := os.Remove(file); err != nil {
				logger.Error(ctx, "failed to remove stale file %s: %v", file, err)
			}
			continue
		}
		// File is alive, increment the count
		aliveCount++
	}

	return aliveCount, nil
}

// Get implements models.ProcStorage.
func (s *Storage) Get(ctx context.Context, workflow digraph.WorkflowRef) (models.Proc, error) {
	panic("unimplemented")
}
