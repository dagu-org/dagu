package localproc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

type ProcFiles struct {
	name      string
	baseDir   string
	staleTime time.Duration
	mu        sync.Mutex
}

func NewProcFiles(baseDir, name string) *ProcFiles {
	return &ProcFiles{
		baseDir:   baseDir,
		name:      name,
		staleTime: time.Second * 45,
	}
}

func (pf *ProcFiles) Count(ctx context.Context, name string) (int, error) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	// Grep for all proc files in the directory
	files, err := filepath.Glob(filepath.Join(pf.baseDir, procFilePrefix+"*.proc"))
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
		if time.Since(fileInfo.ModTime()) > pf.staleTime {
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

func (pf *ProcFiles) GetProc(ctx context.Context, workflow digraph.WorkflowRef) (*Proc, error) {
	// Sanity check the workflow reference
	if pf.name != workflow.Name {
		return nil, fmt.Errorf("workflow name %s does not match proc file name %s", workflow.Name, pf.name)
	}
	// Generate the proc file name
	fileName := pf.getFileName(models.NewUTC(time.Now()), workflow)
	return NewProc(fileName), nil
}

// getFileName generates a proc file name based on the workflow reference.
func (pf *ProcFiles) getFileName(t models.TimeInUTC, workflow digraph.WorkflowRef) string {
	timestamp := t.Format(dateTimeFormatUTC)
	fileName := procFilePrefix + timestamp + "Z_" + workflow.WorkflowID + ".proc"
	return filepath.Join(pf.baseDir, fileName)
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"
