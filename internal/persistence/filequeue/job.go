package filequeue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/core/execution"
)

var _ execution.QueuedItemData = (*Job)(nil)

type Job struct {
	id   string
	file string
	ItemData
}

func NewJob(file string, data ItemData) *Job {
	base := filepath.Base(data.FileName)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return &Job{
		file:     file,
		id:       name,
		ItemData: data,
	}
}

// ID implements models.QueuedJob.
func (j *Job) ID() string {
	return j.id
}

// Data implements models.QueuedItem.
func (j *Job) Data() (*execution.DAGRunRef, error) {
	var itemData ItemData

	fileData, err := os.ReadFile(j.file) // nolint: gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read queue file %s: %w", j.file, err)
	}

	if err := json.Unmarshal(fileData, &itemData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue file %s: %w", j.file, err)
	}

	return &itemData.DAGRun, nil
}
