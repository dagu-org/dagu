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

	// Create a DAG with fields that would be set from dag.json (JSON-serialized fields)
	// These fields are inherited from base.yaml and stored in dag.json
	dag := &core.DAG{
		Name:           "test-dag",
		Queue:          "Default",                            // Inherited from base.yaml
		WorkerSelector: map[string]string{"env": "prod"},     // Inherited from base.yaml
		MaxActiveRuns:  5,                                    // Inherited from base.yaml
		MaxActiveSteps: 3,                                    // Inherited from base.yaml
		LogDir:         "/custom/logs",                       // Inherited from base.yaml
		Tags:           []string{"important", "production"},  // Inherited from base.yaml
		Location:       "/path/to/dag.yaml",
		YamlData:       []byte("steps:\n  - name: test\n    command: echo hello"),
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	// Verify all JSON-serialized fields are preserved
	assert.Equal(t, "Default", result.Queue, "Queue should be preserved")
	assert.Equal(t, map[string]string{"env": "prod"}, result.WorkerSelector, "WorkerSelector should be preserved")
	assert.Equal(t, 5, result.MaxActiveRuns, "MaxActiveRuns should be preserved")
	assert.Equal(t, 3, result.MaxActiveSteps, "MaxActiveSteps should be preserved")
	assert.Equal(t, "/custom/logs", result.LogDir, "LogDir should be preserved")
	assert.Equal(t, []string{"important", "production"}, result.Tags, "Tags should be preserved")
	assert.Equal(t, "/path/to/dag.yaml", result.Location, "Location should be preserved")

	// Verify the original dag pointer is returned (not a new DAG)
	assert.Same(t, dag, result, "Should return the same DAG pointer, not a new one")
}

func TestRebuildDAGFromYAML_EmptyYAML(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		Queue:    "Default",
		YamlData: nil, // Empty YAML
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	// Should return the original DAG unchanged
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

	// Queue should still be preserved
	assert.Equal(t, "Default", result.Queue)

	// Env should be rebuilt from YAML
	assert.Contains(t, result.Env, "MY_VAR=my_value", "Env should be rebuilt from YAML")
}
