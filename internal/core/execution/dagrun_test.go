package execution_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
)

func TestListDAGRunStatusesOptions(t *testing.T) {
	from := execution.NewUTC(time.Now().Add(-24 * time.Hour))
	to := execution.NewUTC(time.Now())
	statuses := []core.Status{core.Succeeded, core.Failed}

	opts := execution.ListDAGRunStatusesOptions{}

	// Apply options
	execution.WithFrom(from)(&opts)
	execution.WithTo(to)(&opts)
	execution.WithStatuses(statuses)(&opts)
	execution.WithExactName("test-dag")(&opts)
	execution.WithName("partial-name")(&opts)
	execution.WithDAGRunID("run-123")(&opts)

	// Verify options were set correctly
	assert.Equal(t, from, opts.From)
	assert.Equal(t, to, opts.To)
	assert.Equal(t, statuses, opts.Statuses)
	assert.Equal(t, "test-dag", opts.ExactName)
	assert.Equal(t, "partial-name", opts.Name)
	assert.Equal(t, "run-123", opts.DAGRunID)
}

func TestNewDAGRunAttemptOptions(t *testing.T) {
	rootDAGRun := &execution.DAGRunRef{
		Name: "root-dag",
		ID:   "root-run-123",
	}

	opts := execution.NewDAGRunAttemptOptions{
		RootDAGRun: rootDAGRun,
		Retry:      true,
	}

	assert.Equal(t, rootDAGRun, opts.RootDAGRun)
	assert.True(t, opts.Retry)
}
