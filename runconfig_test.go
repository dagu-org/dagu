package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
)

func TestRunConfigParsing(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			// Check if file exists
			if _, err := os.Stat(tt.filename); os.IsNotExist(err) {
				t.Skipf("Test file %s does not exist", tt.filename)
				return
			}

			// Load the DAG
			ctx := context.Background()
			dag, err := digraph.LoadBaseConfig(digraph.BuildContext{}, tt.filename)
			if err != nil {
				t.Fatalf("Failed to load DAG: %v", err)
			}

			// Check runConfig
			if dag.RunConfig == nil {
				t.Fatal("RunConfig is nil")
			}

			if dag.RunConfig.AllowEditParams != tt.expected.AllowEditParams {
				t.Errorf("AllowEditParams: expected %v, got %v",
					tt.expected.AllowEditParams, dag.RunConfig.AllowEditParams)
			}

			if dag.RunConfig.AllowEditRunId != tt.expected.AllowEditRunId {
				t.Errorf("AllowEditRunId: expected %v, got %v",
					tt.expected.AllowEditRunId, dag.RunConfig.AllowEditRunId)
			}

			fmt.Printf("✓ %s: AllowEditParams=%v, AllowEditRunId=%v\n",
				tt.name, dag.RunConfig.AllowEditParams, dag.RunConfig.AllowEditRunId)
		})
	}
}

func main() {
	test := &testing.T{}
	TestRunConfigParsing(test)
}
