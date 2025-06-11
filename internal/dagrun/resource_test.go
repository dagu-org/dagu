package dagrun

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
)

func TestResourceParsing(t *testing.T) {
	t.Run("DAG with resource limits parses correctly", func(t *testing.T) {
		yamlContent := `
name: resource-test
resources:
  requests:
    cpu: "0.5"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
steps:
  - name: test-step
    command: echo "testing resources"
`
		
		// Parse DAG with resources
		dag, err := digraph.LoadYAMLWithOpts(context.Background(), []byte(yamlContent), digraph.BuildOpts{})
		if err != nil {
			t.Fatalf("Failed to load DAG: %v", err)
		}
		
		// Verify DAG has resource configuration
		if dag.Resources == nil {
			t.Fatal("Expected DAG to have resource configuration")
		}
		
		if dag.Resources.CPURequestMillis != 500 {
			t.Errorf("Expected CPU request 500, got %d", dag.Resources.CPURequestMillis)
		}
		
		if dag.Resources.CPULimitMillis != 2000 {
			t.Errorf("Expected CPU limit 2000, got %d", dag.Resources.CPULimitMillis)
		}
		
		if dag.Resources.MemoryRequestBytes != 1073741824 { // 1Gi
			t.Errorf("Expected memory request 1073741824, got %d", dag.Resources.MemoryRequestBytes)
		}
		
		if dag.Resources.MemoryLimitBytes != 4294967296 { // 4Gi
			t.Errorf("Expected memory limit 4294967296, got %d", dag.Resources.MemoryLimitBytes)
		}
		
		t.Logf("Successfully parsed DAG with resource limits - CPU: %dm, Memory: %dM", 
			dag.Resources.CPULimitMillis, dag.Resources.MemoryLimitBytes/(1024*1024))
	})
	
	t.Run("DAG without resources has empty config", func(t *testing.T) {
		yamlContent := `
name: no-resources-test
steps:
  - name: test-step
    command: echo "no resources needed"
`
		
		dag, err := digraph.LoadYAMLWithOpts(context.Background(), []byte(yamlContent), digraph.BuildOpts{})
		if err != nil {
			t.Fatalf("Failed to load DAG: %v", err)
		}
		
		// DAG should have empty resources (not nil, but all zero values)
		if dag.Resources == nil {
			t.Fatal("Expected DAG to have empty resource configuration")
		}
		
		// All values should be zero
		if dag.Resources.CPULimitMillis != 0 || dag.Resources.MemoryLimitBytes != 0 {
			t.Error("Expected empty resources to have zero values")
		}
		
		t.Log("Successfully parsed DAG without resource limits")
	})
}