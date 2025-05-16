package prototype

import (
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueuedItem = (*Job)(nil)

type Job struct {
	ItemData
}

func NewJob(data ItemData) *Job {
	return &Job{
		ItemData: data,
	}
}

// ID implements models.QueuedJob.
func (j *Job) ID() string {
	return j.ItemData.Workflow.WorkflowID
}

// Data implements models.QueuedItem.
func (j *Job) Data() (*digraph.WorkflowRef, error) {
	return &j.ItemData.Workflow, nil
}
