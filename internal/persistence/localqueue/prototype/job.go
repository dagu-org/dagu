package prototype

import (
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueuedItem = (*job)(nil)

type job struct {
	ItemData
}

func NewJob(data ItemData) *job {
	return &job{
		ItemData: data,
	}
}

// ID implements models.QueuedJob.
func (j *job) ID() string {
	return j.ItemData.Workflow.WorkflowID
}

// Data implements models.QueuedItem.
func (j *job) Data() (*digraph.WorkflowRef, error) {
	return &j.ItemData.Workflow, nil
}
