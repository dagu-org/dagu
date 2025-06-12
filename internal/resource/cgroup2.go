package resource

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

// Compile-time check that CgroupsV2Enforcer implements digraph.ResourceEnforcer
var _ digraph.ResourceEnforcer = (*CgroupsV2Enforcer)(nil)

const (
	// CgroupRoot is the default cgroup v2 root path
	CgroupRoot = "/sys/fs/cgroup"

	// CPUPeriodMicros is the default CPU period in microseconds
	CPUPeriodMicros = 100000 // 100ms

	// MemoryViolationThreshold is the threshold for considering memory usage a violation
	MemoryViolationThreshold = 0.95 // 95%

	// CPUDefaultWeight is the default CPU weight (1 CPU = 1000 millicores)
	CPUDefaultWeight = 100
)

// CgroupsV2Enforcer implements resource enforcement using Linux cgroups v2
type CgroupsV2Enforcer struct {
	name       string
	resources  *digraph.Resources
	cgroupPath string
}

// NewCgroupsV2Enforcer creates a new cgroups v2 enforcer
func NewCgroupsV2Enforcer(name string, resources *digraph.Resources) (*CgroupsV2Enforcer, error) {
	// Create a unique cgroup for this DAG run
	safeName := fileutil.SafeName(name)
	cgroupPath := filepath.Join(CgroupRoot, "dagu.slice", fmt.Sprintf("dagrun-%s.scope", safeName))

	// Create the cgroup directory
	if err := os.MkdirAll(cgroupPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable required controllers in parent cgroup
	if err := enableControllers(filepath.Dir(cgroupPath)); err != nil {
		_ = os.RemoveAll(cgroupPath)
		return nil, fmt.Errorf("failed to enable controllers: %w", err)
	}

	return &CgroupsV2Enforcer{
		name:       name,
		resources:  resources,
		cgroupPath: cgroupPath,
	}, nil
}

// PreStart configures resource limits before process starts
func (e *CgroupsV2Enforcer) PreStart(_ *exec.Cmd) error {
	// Set memory limits
	if e.resources.MemoryLimitBytes > 0 {
		if err := writeFile(e.cgroupPath, "memory.max",
			fmt.Sprintf("%d", e.resources.MemoryLimitBytes)); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
	}

	// Set memory requests (soft limit/protection)
	if e.resources.MemoryRequestBytes > 0 {
		// memory.low provides memory protection
		if err := writeFile(e.cgroupPath, "memory.low",
			fmt.Sprintf("%d", e.resources.MemoryRequestBytes)); err != nil {
			// This is a warning only as some kernels don't support memory.low
			fmt.Fprintf(os.Stderr, "Warning: memory.low not supported: %v\n", err)
		}
	}

	// Set CPU limits
	if e.resources.CPULimitMillis > 0 {
		// cpu.max format: "$quota $period"
		// quota is in microseconds, we have millicores
		// 1000 millicores = 1 CPU = 100000 microseconds per 100000 microsecond period
		quota := e.resources.CPULimitMillis * 100 // Convert millicores to microseconds
		period := CPUPeriodMicros

		if err := writeFile(e.cgroupPath, "cpu.max",
			fmt.Sprintf("%d %d", quota, period)); err != nil {
			return fmt.Errorf("failed to set CPU limit: %w", err)
		}
	}

	// Set CPU requests (weight for proportional share)
	if e.resources.CPURequestMillis > 0 {
		// cpu.weight range is 1-10000, default is 100
		// We map millicores to weight: 1000 millicores = weight 100
		weight := calculateCPUWeight(e.resources.CPURequestMillis)
		if err := writeFile(e.cgroupPath, "cpu.weight",
			fmt.Sprintf("%d", weight)); err != nil {
			return fmt.Errorf("failed to set CPU weight: %w", err)
		}
	}

	return nil
}

// PostStart adds the process to the cgroup after it starts
func (e *CgroupsV2Enforcer) PostStart(pid int) error {
	// Add process to cgroup
	return writeFile(e.cgroupPath, "cgroup.procs", fmt.Sprintf("%d", pid))
}

// GetMetrics retrieves current resource usage
func (e *CgroupsV2Enforcer) GetMetrics(_ int) (*digraph.Metrics, error) {
	metrics := &digraph.Metrics{
		Timestamp: time.Now(),
	}

	// Read memory usage
	if data, err := readFile(e.cgroupPath, "memory.current"); err == nil {
		metrics.MemoryUsage, _ = strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	}

	// Read memory limit
	if data, err := readFile(e.cgroupPath, "memory.max"); err == nil {
		if strings.TrimSpace(data) != "max" {
			metrics.MemoryLimit, _ = strconv.ParseInt(strings.TrimSpace(data), 10, 64)
		}
	}

	// Read CPU statistics
	if data, err := readFile(e.cgroupPath, "cpu.stat"); err == nil {
		metrics.CPUUsageMillis = parseCPUStat(data)

		// Check if throttled
		if strings.Contains(data, "nr_throttled") {
			lines := strings.Split(data, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "nr_throttled ") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						throttled, _ := strconv.ParseInt(parts[1], 10, 64)
						metrics.Throttled = throttled > 0
					}
				}
			}
		}
	}

	return metrics, nil
}

// CheckViolation checks if resource limits are being violated
func (e *CgroupsV2Enforcer) CheckViolation(metrics *digraph.Metrics) bool {
	// Memory violations are handled by the kernel (OOM killer)
	// We just report if we're close to the limit
	if e.resources.MemoryLimitBytes > 0 && metrics.MemoryUsage > 0 {
		// Consider it a violation if we're using more than the threshold
		threshold := float64(e.resources.MemoryLimitBytes) * MemoryViolationThreshold
		return float64(metrics.MemoryUsage) > threshold
	}
	return false
}

// Cleanup removes the cgroup
func (e *CgroupsV2Enforcer) Cleanup() error {
	// First, try to kill any remaining processes
	if data, err := readFile(e.cgroupPath, "cgroup.procs"); err == nil {
		pids := strings.Fields(strings.TrimSpace(data))
		for _, pidStr := range pids {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				// Try graceful termination first
				if err := killProcess(pid); err != nil {
					// Log error but continue cleanup
					fmt.Fprintf(os.Stderr, "Warning: failed to kill process %d: %v\n", pid, err)
				}
			}
		}
	}

	// Remove the cgroup
	return os.RemoveAll(e.cgroupPath)
}

func (e *CgroupsV2Enforcer) SupportsRequests() bool { return true }
func (e *CgroupsV2Enforcer) SupportsLimits() bool   { return true }

// Helper functions

// enableControllers enables CPU and memory controllers in the parent cgroup
func enableControllers(parentPath string) error {
	controllersFile := filepath.Join(parentPath, "cgroup.subtree_control")

	// Read current controllers
	current, err := os.ReadFile(controllersFile) // #nosec G304 - controllersFile is constructed from constants
	if err != nil {
		return err
	}

	controllers := string(current)
	modified := false

	// Enable CPU controller if not already enabled
	if !strings.Contains(controllers, "cpu") {
		controllers = strings.TrimSpace(controllers + " +cpu")
		modified = true
	}

	// Enable memory controller if not already enabled
	if !strings.Contains(controllers, "memory") {
		controllers = strings.TrimSpace(controllers + " +memory")
		modified = true
	}

	// Write back if modified
	if modified {
		return os.WriteFile(controllersFile, []byte(controllers), 0600)
	}

	return nil
}

// writeFile writes content to a file in the cgroup
func writeFile(cgroupPath, filename, content string) error {
	path := filepath.Join(cgroupPath, filename)
	return os.WriteFile(path, []byte(content), 0600)
}

// readFile reads content from a file in the cgroup
func readFile(cgroupPath, filename string) (string, error) {
	path := filepath.Join(cgroupPath, filename)
	data, err := os.ReadFile(path) // #nosec G304 - path is constructed from cgroupPath
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// calculateCPUWeight converts millicores to cgroup v2 CPU weight
func calculateCPUWeight(millicores int) int {
	// Default weight is 100 for 1 CPU (1000 millicores)
	// Scale proportionally: weight = (millicores / 1000) * CPUDefaultWeight
	// Clamp between 1 and 10000
	weight := (millicores * CPUDefaultWeight) / 1000
	if weight < 1 {
		weight = 1
	}
	if weight > 10000 {
		weight = 10000
	}
	return weight
}

// parseCPUStat parses cpu.stat file and returns usage in milliseconds
func parseCPUStat(data string) int {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "usage_usec ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				usec, err := strconv.ParseInt(parts[1], 10, 64)
				if err == nil {
					return int(usec / 1000) // Convert microseconds to milliseconds
				}
			}
		}
	}
	return 0
}

// killProcess attempts to gracefully terminate a process
func killProcess(pid int) error {
	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(os.Signal(syscall.SIGTERM)); err != nil {
		// If SIGTERM fails, the process might already be dead
		// Check if it's still running before returning error
		if processExists(pid) {
			return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
		}
	}

	return nil
}
