package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildParallel(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		wantItems     int
		wantMaxConc   int
		wantFirstItem any
		wantVariable  string
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "DirectVariableReference",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel: ${ITEMS}
`,
			wantItems:    0,
			wantVariable: "${ITEMS}",
			wantMaxConc:  10, // default
		},
		{
			name: "StaticArray",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      - item1
      - item2
      - item3
`,
			wantItems:     3,
			wantMaxConc:   10, // default
			wantFirstItem: "item1",
		},
		{
			name: "StaticArrayWithObjects",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      - SOURCE: s3://customers
      - SOURCE: s3://products
      - SOURCE: s3://orders
`,
			wantItems:   3,
			wantMaxConc: 10, // default
		},
		{
			name: "StaticArrayWithMaxConcurrent",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      maxConcurrent: 2
      items:
        - SOURCE: s3://customers
        - SOURCE: s3://products
        - SOURCE: s3://orders
`,
			wantItems:   3,
			wantMaxConc: 2,
		},
		{
			name: "ObjectFormWithItemsAndMaxConcurrent",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      items: ${ITEMS}
      maxConcurrent: 5
`,
			wantItems:    0,
			wantVariable: "${ITEMS}",
			wantMaxConc:  5,
		},
		{
			name: "ObjectFormWithStaticArrayAndMaxConcurrent",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      items:
        - item1
        - item2
      maxConcurrent: 3
`,
			wantItems:     2,
			wantMaxConc:   3,
			wantFirstItem: "item1",
		},
		{
			name: "ErrorParallelWithoutRunField",
			yaml: `
steps:
  - name: process
    command: echo test
    parallel: ${ITEMS}
`,
			wantErr:    true,
			wantErrMsg: "parallel execution is only supported for child-DAGs",
		},
		{
			name: "ErrorParallelWithoutCommandOrRun",
			yaml: `
steps:
  - name: process
    parallel: ${ITEMS}
`,
			wantErr: true,
		},
		{
			name: "ErrorInvalidMaxConcurrent",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      items: [1, 2, 3]
      maxConcurrent: 0
`,
			wantErr:    true,
			wantErrMsg: "maxConcurrent must be greater than 0",
		},
		{
			name: "ErrorEmptyItems",
			yaml: `
steps:
  - name: process
    run: workflows/processor
    parallel:
      items: []
      maxConcurrent: 5
`,
			wantErr:    true,
			wantErrMsg: "parallel must have either items array or variable reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the YAML content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.yaml), 0644)
			require.NoError(t, err)

			// Load the DAG
			dag, err := builder.Load(context.Background(), tmpFile)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, dag)
			require.Len(t, dag.Steps, 1)

			step := dag.Steps[0]
			require.NotNil(t, step.Parallel)

			assert.Equal(t, tt.wantItems, len(step.Parallel.Items))
			assert.Equal(t, tt.wantMaxConc, step.Parallel.MaxConcurrent)

			if tt.wantVariable != "" {
				assert.Equal(t, tt.wantVariable, step.Parallel.Variable)
			}

			if tt.wantFirstItem != nil && len(step.Parallel.Items) > 0 {
				// Check the first item's value
				firstItem := step.Parallel.Items[0]
				if firstItem.Value != "" {
					assert.Equal(t, tt.wantFirstItem, firstItem.Value)
				} else {
					// For simple strings in tests, we expect Value to be set
					assert.Equal(t, tt.wantFirstItem, firstItem.Value)
				}
			}
		})
	}
}

func TestParallelWithChildDAG(t *testing.T) {
	yaml := `
steps:
  - name: process-regions
    run: workflows/deploy
    parallel:
      - REGION: us-east-1
      - REGION: eu-west-1
      - REGION: ap-south-1
    params: VERSION=1.0.0
`
	// Create a temporary file with the YAML content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(tmpFile, []byte(yaml), 0644)
	require.NoError(t, err)

	// Load the DAG
	dag, err := builder.Load(context.Background(), tmpFile)
	require.NoError(t, err)
	require.NotNil(t, dag)
	require.Len(t, dag.Steps, 1)

	step := dag.Steps[0]
	require.NotNil(t, step.Parallel)
	require.NotNil(t, step.ChildDAG)

	assert.Equal(t, 3, len(step.Parallel.Items))
	assert.Equal(t, 10, step.Parallel.MaxConcurrent)
	assert.Equal(t, "workflows/deploy", step.ChildDAG.Name)
	assert.Equal(t, "VERSION=\"1.0.0\"", step.ChildDAG.Params)

	// Check the items
	items := step.Parallel.Items
	assert.NotNil(t, items[0].Params)
	assert.Equal(t, "us-east-1", items[0].Params["REGION"])
}

func TestParallelIntegration(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "ParallelWithEnvVariableSubstitution",
			yaml: `
env:
  - REGIONS: '["us-east-1", "eu-west-1"]'
steps:
  - name: deploy-all
    run: workflows/deploy
    parallel: ${REGIONS}
`,
		},
		{
			name: "ParallelWithOutputFromPreviousStep",
			yaml: `
steps:
  - name: get-items
    command: echo '["item1", "item2", "item3"]'
    output: ITEMS
    
  - name: process-all
    run: workflows/processor
    parallel: ${ITEMS}
    depends: get-items
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the YAML content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.yaml), 0644)
			require.NoError(t, err)

			// Load the DAG - just verify it parses correctly
			dag, err := builder.Load(context.Background(), tmpFile)
			require.NoError(t, err)
			require.NotNil(t, dag)
		})
	}
}
