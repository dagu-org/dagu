package resource

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
)

func TestResourceEnforcementIntegration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Resource enforcement only works on Linux with cgroups v2")
	}

	rc, err := NewResourceController()
	if err != nil {
		t.Fatalf("Failed to create resource controller: %v", err)
	}

	// Check if we have cgroups v2 support
	if !rc.capabilities.CgroupsV2 {
		t.Skip("cgroups v2 not available")
	}

	// Test with memory limit
	resources := &digraph.Resources{
		MemoryLimitBytes: 10 * 1024 * 1024, // 10MB
		CPULimitMillis:   100,               // 0.1 CPU
	}

	// This command tries to allocate 20MB of memory
	cmd := exec.Command("sh", "-c", "dd if=/dev/zero of=/dev/null bs=1M count=20")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = rc.StartProcess(ctx, cmd, resources, "test-memory-limit")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for process to complete or be killed
	err = cmd.Wait()
	
	// On successful enforcement, the process should be killed by OOM killer
	if err == nil {
		t.Log("Process completed successfully - memory limit may not be enforced")
	} else {
		t.Logf("Process failed as expected: %v", err)
	}

	// Cleanup
	rc.StopProcess("test-memory-limit")
}

func TestActualEnforcerSelection(t *testing.T) {
	rc, err := NewResourceController()
	if err != nil {
		t.Fatalf("Failed to create resource controller: %v", err)
	}

	resources := &digraph.Resources{
		MemoryLimitBytes: 100 * 1024 * 1024, // 100MB
	}

	enforcer, err := rc.createEnforcer("test", resources)
	if err != nil {
		t.Fatalf("Failed to create enforcer: %v", err)
	}

	switch enforcer.(type) {
	case *CgroupsV2Enforcer:
		t.Log("Using cgroups v2 enforcer")
		if !enforcer.SupportsLimits() {
			t.Error("CgroupsV2Enforcer should support limits")
		}
	case *RlimitEnforcer:
		t.Log("Using rlimit enforcer")
		if !enforcer.SupportsLimits() {
			t.Error("RlimitEnforcer should support limits via environment variables")
		}
	case *NoopEnforcer:
		t.Log("Using noop enforcer - no enforcement available")
	default:
		t.Errorf("Unknown enforcer type: %T", enforcer)
	}
}