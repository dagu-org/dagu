package digraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDAG_FileName(t *testing.T) {
	tests := []struct {
		name         string
		dag          *DAG
		expectedName string
	}{
		{
			name: "with fileName set",
			dag: &DAG{
				Location: "/path/to/workflow/task1.yaml",
				fileName: "workflow/task1",
			},
			expectedName: "workflow/task1",
		},
		{
			name: "without fileName set - backward compatibility",
			dag: &DAG{
				Location: "/path/to/task1.yaml",
			},
			expectedName: "task1",
		},
		{
			name: "without fileName set - with yml extension",
			dag: &DAG{
				Location: "/path/to/task1.yml",
			},
			expectedName: "task1.yaml",
		},
		{
			name: "empty location",
			dag: &DAG{
				Location: "",
			},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dag.FileName()
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestDAG_SetFileName(t *testing.T) {
	dag := &DAG{
		Location: "/path/to/task1.yaml",
	}

	// Initially should use location
	assert.Equal(t, "task1", dag.FileName())

	// Set prefixed name
	dag.SetFileName("workflow/task1")
	assert.Equal(t, "workflow/task1", dag.FileName())

	// Set another name
	dag.SetFileName("data/pipeline/process")
	assert.Equal(t, "data/pipeline/process", dag.FileName())
}