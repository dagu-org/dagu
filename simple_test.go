package main

import (
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/digraph"
)

func main() {
	tests := []struct {
		name     string
		filename string
		expected *digraph.RunConfig
	}{
		{
			name:     "Restricted parameters and run ID",
			filename: "test-dag-restricted.yaml",
			expected: &digraph.RunConfig{
				AllowEditParams: false,
				AllowEditRunId:  false,
			},
		},
		{
			name:     "Unrestricted (default behavior)",
			filename: "test-dag-unrestricted.yaml",
			expected: &digraph.RunConfig{
				AllowEditParams: true,
				AllowEditRunId:  true,
			},
		},
		{
			name:     "Partial restriction",
			filename: "test-dag-partial-restriction.yaml",
			expected: &digraph.RunConfig{
				AllowEditParams: true,
				AllowEditRunId:  false,
			},
		},
	}

	for _, tt := range tests {
		fmt.Printf("Testing %s...\n", tt.name)

		// Check if file exists
		if _, err := os.Stat(tt.filename); os.IsNotExist(err) {
			fmt.Printf("  SKIP: Test file %s does not exist\n", tt.filename)
			continue
		}

		// Load the DAG
		dag, err := digraph.LoadBaseConfig(digraph.BuildContext{}, tt.filename)
		if err != nil {
			fmt.Printf("  ERROR: Failed to load DAG: %v\n", err)
			continue
		}

		// Check runConfig
		if dag.RunConfig == nil {
			fmt.Printf("  ERROR: RunConfig is nil\n")
			continue
		}

		success := true

		if dag.RunConfig.AllowEditParams != tt.expected.AllowEditParams {
			fmt.Printf("  ERROR: AllowEditParams: expected %v, got %v\n",
				tt.expected.AllowEditParams, dag.RunConfig.AllowEditParams)
			success = false
		}

		if dag.RunConfig.AllowEditRunId != tt.expected.AllowEditRunId {
			fmt.Printf("  ERROR: AllowEditRunId: expected %v, got %v\n",
				tt.expected.AllowEditRunId, dag.RunConfig.AllowEditRunId)
			success = false
		}

		if success {
			fmt.Printf("  ✓ PASS: AllowEditParams=%v, AllowEditRunId=%v\n",
				dag.RunConfig.AllowEditParams, dag.RunConfig.AllowEditRunId)
		}
	}
}
