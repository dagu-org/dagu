package filequeue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/core/execution"
)

var _ execution.QueuedItemData = (*QueuedFile)(nil)

type QueuedFile struct {
	id   string
	file string

	cache *ItemData
	lock  sync.Mutex
}

func NewQueuedFile(file string) *QueuedFile {
	base := filepath.Base(file)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return &QueuedFile{
		file: file,
		id:   name,
	}
}

// ID implements execution.QueuedItemData.
func (j *QueuedFile) ID() string {
	return j.id
}

// Data implements execution.QueuedItemData.
func (j *QueuedFile) Data() (*execution.DAGRunRef, error) {
	itemData, err := j.loadData()
	if err != nil {
		return nil, fmt.Errorf("failed to load job data: %w", err)
	}
	return &itemData.DAGRun, nil
}

func (j *QueuedFile) loadData() (*ItemData, error) {
	j.lock.Lock()
	defer j.lock.Unlock()

	if j.cache != nil {
		return j.cache, nil
	}

	var itemData ItemData

	fileData, err := os.ReadFile(j.file) // nolint: gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read queue file %s: %w", j.file, err)
	}

	if err := json.Unmarshal(fileData, &itemData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue file %s: %w", j.file, err)
	}

	j.cache = &itemData

	return &itemData, nil
}

// ExtractJob loads and returns the underlying QueuedItemData.
func (j *QueuedFile) ExtractJob() (*Job, error) {
	data, err := j.loadData()
	if err != nil {
		return nil, fmt.Errorf("failed to load job data: %w", err)
	}

	return &Job{
		id:       j.id,
		ItemData: *data,
	}, nil
}

var _ execution.QueuedItemData = (*Job)(nil)

// Job implements execution.QueuedItemData for a job stored in a file.
type Job struct {
	id string
	ItemData
}

func (j *Job) ID() string {
	return j.id
}

func (j *Job) Data() (*execution.DAGRunRef, error) {
	return &j.DAGRun, nil
}
