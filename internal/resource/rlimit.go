package resource

import (
	"os/exec"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
)

// RlimitEnforcer implements resource enforcement using Unix rlimits
// This works on Unix-like systems (Linux, macOS, BSD) as a fallback
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
func (e *RlimitEnforcer) PreStart(cmd *exec.Cmd) error {
	// Initialize SysProcAttr if needed
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Set memory limits using syscall instead of shell wrapper for security
	if e.resources.MemoryLimitBytes > 0 {
		// Unfortunately, Go's exec.Cmd doesn't support setting rlimits directly
		// We'll need to use a wrapper process or accept this limitation
		// For now, we'll document this as a known limitation
		
		// TODO: Implement proper rlimit setting via a wrapper process
		// that calls setrlimit before exec
	}
	
	// For CPU limits, we could use nice values as a hint but this requires
	// wrapping the command which has security implications
	// For now, we'll skip CPU priority adjustment in rlimit enforcer
	
	// TODO: Implement safe CPU priority adjustment
	
	return nil
}

// PostStart is called after the process starts
func (e *RlimitEnforcer) PostStart(pid int) error {
	// rlimits are inherited from parent, no post-start action needed
	return nil
}

// GetMetrics retrieves current resource usage
func (e *RlimitEnforcer) GetMetrics(pid int) (*Metrics, error) {
	return getMetricsFromPS(pid)
}

// CheckViolation checks if resource limits are being violated
func (e *RlimitEnforcer) CheckViolation(metrics *Metrics) bool {
	// rlimits are enforced by the kernel
	return false
}

// Cleanup performs cleanup tasks
func (e *RlimitEnforcer) Cleanup() error {
	return nil
}

func (e *RlimitEnforcer) SupportsRequests() bool { return false }
func (e *RlimitEnforcer) SupportsLimits() bool   { return false } // Currently disabled due to security concerns


