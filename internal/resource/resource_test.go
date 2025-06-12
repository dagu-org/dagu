package resource

import (
	"context"
	"os/exec"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
)

func TestNewResourceController(t *testing.T) {
	rc, err := NewResourceController()
	if err != nil {
		t.Fatalf("Failed to create resource controller: %v", err)
	}

	if rc == nil {
		t.Fatal("Resource controller is nil")
	}

	if rc.enforcers == nil {
		t.Fatal("Enforcers map is nil")
	}
}

func TestStartProcessWithoutResources(t *testing.T) {
	rc, err := NewResourceController()
	if err != nil {
		t.Fatalf("Failed to create resource controller: %v", err)
	}

	// Test with nil resources
	cmd := exec.Command("echo", "test")
	err = rc.StartProcess(context.Background(), cmd, nil, "test-nil")
	if err != nil {
		t.Fatalf("Failed to start process with nil resources: %v", err)
	}

	// Wait for command to complete
	_ = cmd.Wait()

	// Test with empty resources
	resources := &digraph.Resources{}
	cmd = exec.Command("echo", "test")
	err = rc.StartProcess(context.Background(), cmd, resources, "test-empty")
	if err != nil {
		t.Fatalf("Failed to start process with empty resources: %v", err)
	}

	// Wait for command to complete
	_ = cmd.Wait()
}

func TestDetectCapabilities(t *testing.T) {
	caps := detectCapabilities()

	// Should have at least rlimit support
	if !caps.Rlimit {
		t.Error("Expected rlimit support to be available")
	}

	// Log detected capabilities for debugging
	t.Logf("Detected capabilities: CgroupsV2=%v, CgroupsV1=%v, Rlimit=%v, Nice=%v",
		caps.CgroupsV2, caps.CgroupsV1, caps.Rlimit, caps.Nice)
}

func TestNoopEnforcer(t *testing.T) {
	enforcer := NewNoopEnforcer()

	// Test all methods
	if err := enforcer.PreStart(nil); err != nil {
		t.Errorf("NoopEnforcer.PreStart failed: %v", err)
	}

	if err := enforcer.PostStart(123); err != nil {
		t.Errorf("NoopEnforcer.PostStart failed: %v", err)
	}

	metrics, err := enforcer.GetMetrics(123)
	if err != nil {
		t.Errorf("NoopEnforcer.GetMetrics failed: %v", err)
	}
	if metrics == nil {
		t.Error("NoopEnforcer.GetMetrics returned nil metrics")
	}

	if enforcer.CheckViolation(metrics) {
		t.Error("NoopEnforcer.CheckViolation should always return false")
	}

	if err := enforcer.Cleanup(); err != nil {
		t.Errorf("NoopEnforcer.Cleanup failed: %v", err)
	}

	if enforcer.SupportsRequests() {
		t.Error("NoopEnforcer should not support requests")
	}

	if enforcer.SupportsLimits() {
		t.Error("NoopEnforcer should not support limits")
	}
}
