package digraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDAGWithResources(t *testing.T) {
	ctx := context.Background()
	
	dag, err := Load(ctx, "../testdata/digraph/resources_test.yaml")
	require.NoError(t, err)
	require.NotNil(t, dag)
	
	// Check DAG-level resources
	require.NotNil(t, dag.Resources)
	assert.Equal(t, 500, dag.Resources.CPURequestMillis)      // 0.5 cores
	assert.Equal(t, int64(Gibibyte), dag.Resources.MemoryRequestBytes) // 1Gi
	assert.Equal(t, 2000, dag.Resources.CPULimitMillis)       // 2 cores
	assert.Equal(t, int64(4*Gibibyte), dag.Resources.MemoryLimitBytes) // 4Gi
	
	// Check steps
	require.Len(t, dag.Steps, 3)
	
	// Light task - no resources
	assert.Equal(t, "light-task", dag.Steps[0].Name)
	assert.NotNil(t, dag.Steps[0].Resources) // Will be empty Resources struct
	assert.Equal(t, 0, dag.Steps[0].Resources.CPULimitMillis)
	
	// Heavy task - only limits
	assert.Equal(t, "heavy-task", dag.Steps[1].Name)
	require.NotNil(t, dag.Steps[1].Resources)
	assert.Equal(t, 0, dag.Steps[1].Resources.CPURequestMillis) // No requests
	assert.Equal(t, 4000, dag.Steps[1].Resources.CPULimitMillis) // 4 cores
	assert.Equal(t, int64(8*Gibibyte), dag.Steps[1].Resources.MemoryLimitBytes) // 8Gi
	
	// Medium task - both requests and limits
	assert.Equal(t, "medium-task", dag.Steps[2].Name)
	require.NotNil(t, dag.Steps[2].Resources)
	assert.Equal(t, 1000, dag.Steps[2].Resources.CPURequestMillis) // 1 core
	assert.Equal(t, int64(2*Gibibyte), dag.Steps[2].Resources.MemoryRequestBytes) // 2Gi
	assert.Equal(t, 2000, dag.Steps[2].Resources.CPULimitMillis) // 2 cores
	assert.Equal(t, int64(4*Gibibyte), dag.Steps[2].Resources.MemoryLimitBytes) // 4Gi
}

func TestLoadDAGWithInvalidResources(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "invalid CPU format",
			yaml: `
name: invalid-cpu
resources:
  limits:
    cpu: "abc"
`,
			wantErr: "invalid CPU quantity",
		},
		{
			name: "invalid memory format",
			yaml: `
name: invalid-memory
resources:
  limits:
    memory: "100X"
`,
			wantErr: "unknown unit",
		},
		{
			name: "invalid step resources",
			yaml: `
name: invalid-step-resources
steps:
  - name: task
    command: echo test
    resources:
      requests:
        cpu: "2x"
`,
			wantErr: "invalid CPU quantity",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := LoadYAMLWithOpts(ctx, []byte(tt.yaml), BuildOpts{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestToResourcesConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *Resources
		expected *ResourcesConfig
	}{
		{
			name:     "nil resources",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty resources",
			input: &Resources{},
			expected: &ResourcesConfig{},
		},
		{
			name: "full resources",
			input: &Resources{
				CPURequestMillis:   500,
				MemoryRequestBytes: Gibibyte,
				CPULimitMillis:     2000,
				MemoryLimitBytes:   4 * Gibibyte,
			},
			expected: &ResourcesConfig{
				Requests: &ResourceQuantities{
					CPU:    "0.5",
					Memory: "1Gi",
				},
				Limits: &ResourceQuantities{
					CPU:    "2",
					Memory: "4Gi",
				},
			},
		},
		{
			name: "only limits",
			input: &Resources{
				CPULimitMillis:   4000,
				MemoryLimitBytes: 8 * Gibibyte,
			},
			expected: &ResourcesConfig{
				Limits: &ResourceQuantities{
					CPU:    "4",
					Memory: "8Gi",
				},
			},
		},
		{
			name: "millicores",
			input: &Resources{
				CPURequestMillis: 250,
				CPULimitMillis:   1750,
			},
			expected: &ResourcesConfig{
				Requests: &ResourceQuantities{
					CPU: "0.25",
				},
				Limits: &ResourceQuantities{
					CPU: "1.75",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.ToResourcesConfig()
			assert.Equal(t, tt.expected, got)
		})
	}
}