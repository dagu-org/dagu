package cmd

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebuildDAGFromYAML_PreservesJSONSerializedFields(t *testing.T) {
	t.Parallel()

	// Create a DAG with JSON-serialized fields (typically inherited from base.yaml)
	dag := &core.DAG{
		Name:           "test-dag",
		Queue:          "Default",
		WorkerSelector: map[string]string{"env": "prod"},
		MaxActiveRuns:  5,
		MaxActiveSteps: 3,
		LogDir:         "/custom/logs",
		Tags:           []string{"important", "production"},
		Location:       "/path/to/dag.yaml",
		YamlData:       []byte("steps:\n  - name: test\n    command: echo hello"),
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	// Verify JSON-serialized fields are preserved
	assert.Equal(t, "Default", result.Queue)
	assert.Equal(t, map[string]string{"env": "prod"}, result.WorkerSelector)
	assert.Equal(t, 5, result.MaxActiveRuns)
	assert.Equal(t, 3, result.MaxActiveSteps)
	assert.Equal(t, "/custom/logs", result.LogDir)
	assert.Equal(t, []string{"important", "production"}, result.Tags)
	assert.Equal(t, "/path/to/dag.yaml", result.Location)

	// Verify the original DAG pointer is returned (not a new DAG)
	assert.Same(t, dag, result)
}

func TestRebuildDAGFromYAML_EmptyYAML(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		Queue:    "Default",
		YamlData: nil,
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	assert.Same(t, dag, result)
	assert.Equal(t, "Default", result.Queue)
}

func TestRebuildDAGFromYAML_RebuildEnvFromYAML(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		Queue:    "Default",
		Location: "/path/to/dag.yaml",
		YamlData: []byte("env:\n  - MY_VAR: my_value\nsteps:\n  - name: test\n    command: echo $MY_VAR"),
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	assert.Equal(t, "Default", result.Queue)
	assert.Contains(t, result.Env, "MY_VAR=my_value")
}
