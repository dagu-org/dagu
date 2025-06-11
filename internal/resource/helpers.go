package resource

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
)

// detectCapabilities detects platform-specific features available
func detectCapabilities() Capabilities {
	caps := Capabilities{}

	switch runtime.GOOS {
	case "linux":
		caps.CgroupsV2 = fileExists("/sys/fs/cgroup/cgroup.controllers")
		caps.CgroupsV1 = fileExists("/sys/fs/cgroup/memory")
		caps.Rlimit = true
		caps.Nice = commandExists("nice")

	case "darwin", "freebsd", "openbsd", "netbsd":
		caps.Rlimit = true
		caps.Nice = commandExists("nice")

	default:
		// Other platforms get basic rlimit support
		caps.Rlimit = true
	}

	return caps
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// processExists checks if a process with given PID exists
func processExists(pid int) bool {
	// Unix systems: send signal 0 to check if process exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// getMetricsFromPS retrieves process metrics using ps command
func getMetricsFromPS(pid int) (*digraph.Metrics, error) {
	metrics := &digraph.Metrics{
		Timestamp: time.Now(),
	}

	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin":
		// macOS ps command format
		cmd = exec.Command("ps", "-o", "rss=,vsz=,%cpu=", "-p", strconv.Itoa(pid))
	default:
		// Linux/BSD ps command format
		cmd = exec.Command("ps", "-o", "rss=,vsz=,%cpu=", "-p", strconv.Itoa(pid))
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get process info: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) >= 3 {
		// RSS in KB
		if rss, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			metrics.MemoryUsage = rss * 1024 // Convert to bytes
		}
		// CPU percentage - convert to milliseconds
		// This is a rough estimate, not exact CPU time
		cpuStr := strings.TrimSuffix(fields[2], "%")
		if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			// Very rough conversion: assume 1% CPU = 10ms per second
			metrics.CPUUsageMillis = int(cpu * 10)
		}
	}

	return metrics, nil
}