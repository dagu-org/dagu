// Package resource provides platform-agnostic resource management for processes.
// It supports CPU and memory limits through various enforcement mechanisms:
// - Linux: cgroups v2 (preferred) or v1
// - Unix: rlimits (limited support)
// - Others: monitoring only
//
// The package handles resource allocation, monitoring, and enforcement
// in a safe and efficient manner across different platforms.
package resource

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
)

const (
	// MonitoringInterval is the interval for resource monitoring
	MonitoringInterval = 10 * time.Second
)

// Capabilities represents platform-specific features available
type Capabilities struct {
	CgroupsV2 bool // Linux cgroups v2 support
	CgroupsV1 bool // Linux cgroups v1 support
	Rlimit    bool // Unix resource limits support
	Nice      bool // Unix nice command support
}

// ResourceController manages resource enforcement across different platforms
type ResourceController struct {
	capabilities Capabilities
	enforcers    map[string]ResourceEnforcer
	mu           sync.RWMutex
}

// NewResourceController creates a new resource controller with detected capabilities
func NewResourceController() (*ResourceController, error) {
	caps := detectCapabilities()

	return &ResourceController{
		capabilities: caps,
		enforcers:    make(map[string]ResourceEnforcer),
	}, nil
}

// StartProcess starts a process with resource limits applied
func (rc *ResourceController) StartProcess(
	ctx context.Context,
	cmd *exec.Cmd,
	resources *digraph.Resources,
	name string,
) error {
	// Skip if no resources specified
	if resources == nil || (resources.CPULimitMillis == 0 && 
		resources.MemoryLimitBytes == 0 &&
		resources.CPURequestMillis == 0 && 
		resources.MemoryRequestBytes == 0) {
		return cmd.Start()
	}

	// Create the most suitable enforcer for the platform
	enforcer, err := rc.createEnforcer(name, resources)
	if err != nil {
		return fmt.Errorf("failed to create enforcer: %w", err)
	}

	if enforcer == nil {
		// No resource enforcement available, start without limits
		return cmd.Start()
	}

	// Variable to track if we've registered the enforcer
	registered := false
	
	// Ensure cleanup on error
	defer func() {
		if err != nil && registered {
			rc.mu.Lock()
			delete(rc.enforcers, name)
			rc.mu.Unlock()
			enforcer.Cleanup()
		}
	}()

	// Apply pre-start configuration
	if err := enforcer.PreStart(cmd); err != nil {
		return fmt.Errorf("pre-start failed: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	// Register enforcer only after process has started
	rc.mu.Lock()
	rc.enforcers[name] = enforcer
	registered = true
	rc.mu.Unlock()
	
	// Apply post-start configuration
	if err := enforcer.PostStart(cmd.Process.Pid); err != nil {
		// Kill the process if post-start fails
		cmd.Process.Kill()
		return fmt.Errorf("post-start failed: %w", err)
	}

	// Start monitoring in background
	go rc.monitorProcess(ctx, name, cmd.Process.Pid, enforcer)

	return nil
}

// StopProcess stops a process and cleans up its resources
func (rc *ResourceController) StopProcess(name string) error {
	rc.mu.Lock()
	enforcer, exists := rc.enforcers[name]
	delete(rc.enforcers, name)
	rc.mu.Unlock()

	if exists && enforcer != nil {
		return enforcer.Cleanup()
	}
	return nil
}

// GetMetrics returns current resource metrics for a process
func (rc *ResourceController) GetMetrics(name string, pid int) (*Metrics, error) {
	rc.mu.RLock()
	enforcer, exists := rc.enforcers[name]
	rc.mu.RUnlock()

	if !exists || enforcer == nil {
		return nil, fmt.Errorf("no enforcer found for %s", name)
	}

	return enforcer.GetMetrics(pid)
}

// createEnforcer selects the best enforcer based on platform capabilities
func (rc *ResourceController) createEnforcer(name string, resources *digraph.Resources) (ResourceEnforcer, error) {
	// Validate resources
	if resources == nil {
		return nil, fmt.Errorf("resources cannot be nil")
	}
	
	switch {
	case rc.capabilities.CgroupsV2 && runtime.GOOS == "linux":
		// Prefer cgroups v2 on Linux
		return NewCgroupsV2Enforcer(name, resources)
		
	case rc.capabilities.CgroupsV1 && runtime.GOOS == "linux":
		// Fall back to cgroups v1 - use rlimit for now
		// TODO: Implement proper cgroups v1 support
		return NewRlimitEnforcer(name, resources)
		
	case rc.capabilities.Rlimit:
		// Use rlimit on Unix-like systems
		return NewRlimitEnforcer(name, resources)
		
	default:
		// No enforcement available
		return NewNoopEnforcer(), nil
	}
}

// monitorProcess monitors a process and collects metrics
func (rc *ResourceController) monitorProcess(
	ctx context.Context,
	name string,
	pid int,
	enforcer ResourceEnforcer,
) {
	// Ensure cleanup when monitoring ends
	defer func() {
		rc.mu.Lock()
		// Only delete if we're still the registered enforcer
		if currentEnforcer, exists := rc.enforcers[name]; exists && currentEnforcer == enforcer {
			delete(rc.enforcers, name)
		}
		rc.mu.Unlock()
		
		// Always cleanup our enforcer
		if enforcer != nil {
			enforcer.Cleanup()
		}
	}()

	ticker := time.NewTicker(MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
			
		case <-ticker.C:
			// Check if process still exists
			if !processExists(pid) {
				return
			}

			// Collect metrics
			metrics, err := enforcer.GetMetrics(pid)
			if err != nil {
				// Log error but continue monitoring
				continue
			}

			// Check for resource violations
			if enforcer.CheckViolation(metrics) {
				// For now, just log violations
				// Future: could implement actions like throttling or killing
				fmt.Printf("Resource violation detected for %s: %+v\n", name, metrics)
			}
		}
	}
}

// NoopEnforcer provides a no-op implementation when resource enforcement is not available
type NoopEnforcer struct{}

func NewNoopEnforcer() *NoopEnforcer {
	return &NoopEnforcer{}
}

func (n *NoopEnforcer) PreStart(cmd *exec.Cmd) error                  { return nil }
func (n *NoopEnforcer) PostStart(pid int) error                       { return nil }
func (n *NoopEnforcer) GetMetrics(pid int) (*Metrics, error)          { return &Metrics{}, nil }
func (n *NoopEnforcer) CheckViolation(metrics *Metrics) bool          { return false }
func (n *NoopEnforcer) Cleanup() error                                { return nil }
func (n *NoopEnforcer) SupportsRequests() bool                        { return false }
func (n *NoopEnforcer) SupportsLimits() bool                          { return false }