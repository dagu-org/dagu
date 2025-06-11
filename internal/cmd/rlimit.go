package cmd

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// applyResourceLimits checks for DAGU_RLIMIT_* environment variables and applies rlimits
// This should be called early in the start command execution
func applyResourceLimits() error {
	// Check for memory limit
	if memLimitStr := os.Getenv("DAGU_RLIMIT_MEMORY"); memLimitStr != "" {
		memLimit, err := strconv.ParseInt(memLimitStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid DAGU_RLIMIT_MEMORY value: %w", err)
		}
		
		if err := setMemoryLimit(memLimit); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
	}
	
	// Check for CPU limit
	if cpuLimitStr := os.Getenv("DAGU_RLIMIT_CPU"); cpuLimitStr != "" {
		cpuLimitMillis, err := strconv.Atoi(cpuLimitStr)
		if err != nil {
			return fmt.Errorf("invalid DAGU_RLIMIT_CPU value: %w", err)
		}
		
		if err := setCPULimit(cpuLimitMillis); err != nil {
			return fmt.Errorf("failed to set CPU limit: %w", err)
		}
	}
	
	return nil
}

// setMemoryLimit sets the virtual memory limit (RLIMIT_AS)
func setMemoryLimit(bytes int64) error {
	const rlimitAS = syscall.RLIMIT_AS
	
	// Get current limit
	var currentLimit syscall.Rlimit
	if err := syscall.Getrlimit(rlimitAS, &currentLimit); err != nil {
		return fmt.Errorf("failed to get current memory limit: %w", err)
	}
	
	// Set new limit
	newLimit := syscall.Rlimit{
		Cur: uint64(bytes),
		Max: currentLimit.Max, // Keep the hard limit unchanged
	}
	
	// Don't allow setting higher than the hard limit
	if newLimit.Cur > currentLimit.Max {
		newLimit.Cur = currentLimit.Max
	}
	
	if err := syscall.Setrlimit(rlimitAS, &newLimit); err != nil {
		return fmt.Errorf("failed to set memory limit: %w", err)
	}
	
	return nil
}

// setCPULimit sets CPU-related limits
// Since we can't directly limit CPU usage percentage with rlimits,
// we use nice value to lower the process priority as a rough approximation
func setCPULimit(milliCores int) error {
	// Convert millicores to nice value
	// 1000 millicores = normal priority (nice 0)
	// 500 millicores = lower priority (nice 10)
	// 100 millicores = very low priority (nice 19)
	
	if milliCores >= 1000 {
		// Full CPU, no need to adjust priority
		return nil
	}
	
	// Calculate nice value: lower millicores = higher nice value (lower priority)
	// Scale: 1000 millicores = 0, 0 millicores = 19
	niceValue := int((1000 - milliCores) * 19 / 1000)
	if niceValue > 19 {
		niceValue = 19
	}
	if niceValue < 0 {
		niceValue = 0
	}
	
	// Set the nice value using syscall
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, niceValue); err != nil {
		// If setting priority fails, it's not critical - continue anyway
		// This might fail due to permissions
		return nil
	}
	
	return nil
}