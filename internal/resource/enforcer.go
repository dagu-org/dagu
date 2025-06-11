package resource

import (
	"os/exec"
	"time"
)

// ResourceEnforcer is an interface that abstracts platform-specific resource enforcement
type ResourceEnforcer interface {
	// PreStart is called before starting the process to set up resource limits
	PreStart(cmd *exec.Cmd) error

	// PostStart is called after the process starts to apply runtime limits
	PostStart(pid int) error

	// GetMetrics retrieves current resource usage metrics
	GetMetrics(pid int) (*Metrics, error)

	// CheckViolation checks if current metrics violate configured limits
	CheckViolation(metrics *Metrics) bool

	// Cleanup removes any created resources (e.g., cgroups)
	Cleanup() error

	// SupportsRequests indicates if the enforcer supports resource requests (reservations)
	SupportsRequests() bool
	
	// SupportsLimits indicates if the enforcer supports hard resource limits
	SupportsLimits() bool
}

// Metrics represents resource usage metrics at a point in time
type Metrics struct {
	Timestamp      time.Time
	MemoryUsage    int64   // Current memory usage in bytes
	MemoryLimit    int64   // Memory limit in bytes (if set)
	CPUUsageMillis int     // Total CPU time used in milliseconds
	CPUPercent     float64 // CPU usage percentage (0-100 * number of cores)
	Throttled      bool    // Whether the process is being throttled
}