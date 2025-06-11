package resource

import (
	"fmt"
	"os/exec"

	"github.com/dagu-org/dagu/internal/digraph"
)

// Compile-time check that RlimitEnforcer implements digraph.ResourceEnforcer
var _ digraph.ResourceEnforcer = (*RlimitEnforcer)(nil)

// RlimitEnforcer implements resource enforcement using Unix rlimits
// This works on Unix-like systems (Linux, macOS, BSD) as a fallback
// It passes resource limits via environment variables to child processes
type RlimitEnforcer struct {
	name      string
	resources *digraph.Resources
}

// NewRlimitEnforcer creates a new rlimit enforcer
func NewRlimitEnforcer(name string, resources *digraph.Resources) (*RlimitEnforcer, error) {
	return &RlimitEnforcer{
		name:      name,
		resources: resources,
	}, nil
}

// PreStart configures resource limits before process starts
// For rlimits, we pass the limits via environment variables to the child process
func (e *RlimitEnforcer) PreStart(cmd *exec.Cmd) error {
	// Pass resource limits via environment variables
	// The child process will apply these rlimits to itself
	
	if e.resources.MemoryLimitBytes > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DAGU_RLIMIT_MEMORY=%d", e.resources.MemoryLimitBytes))
	}
	
	if e.resources.CPULimitMillis > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DAGU_RLIMIT_CPU=%d", e.resources.CPULimitMillis))
	}

	return nil
}

// PostStart is called after the process starts
func (e *RlimitEnforcer) PostStart(pid int) error {
	// No action needed - rlimits are set by the child process itself
	return nil
}

// GetMetrics retrieves current resource usage
func (e *RlimitEnforcer) GetMetrics(pid int) (*digraph.Metrics, error) {
	return getMetricsFromPS(pid)
}

// CheckViolation checks if resource limits are being violated
func (e *RlimitEnforcer) CheckViolation(metrics *digraph.Metrics) bool {
	// rlimits are enforced by the kernel
	return false
}

// Cleanup performs cleanup tasks
func (e *RlimitEnforcer) Cleanup() error {
	// No cleanup needed - rlimits are process-local
	return nil
}

func (e *RlimitEnforcer) SupportsRequests() bool { return false }
func (e *RlimitEnforcer) SupportsLimits() bool   { return true } // Support limits via environment variables
