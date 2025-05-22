package prototype

import (
	"path/filepath"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueuedItem = (*Job)(nil)

type Job struct {
	id string
	ItemData
}

func NewJob(data ItemData) *Job {
	base := filepath.Base(data.FileName)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return &Job{
		id:       name,
		ItemData: data,
	}
}

// ID implements models.QueuedJob.
func (j *Job) ID() string {
	return j.id
}

// Data implements models.QueuedItem.
func (j *Job) Data() digraph.WorkflowRef {
	return j.ItemData.Workflow
}
