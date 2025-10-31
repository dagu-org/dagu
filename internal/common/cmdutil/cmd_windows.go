//go:build windows
// +build windows

package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SetupCommand configures Windows-specific command attributes
func SetupCommand(cmd *exec.Cmd) {
	setupCommand(cmd)
}

// setupCommand configures Windows-specific command attributes
func setupCommand(cmd *exec.Cmd) {
	// Windows doesn't support process groups in the same way as Unix
	// No special configuration needed
}

// KillProcessGroup kills the process on Windows systems
func KillProcessGroup(cmd *exec.Cmd, sig os.Signal) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// KillMultipleProcessGroups kills multiple processes on Windows systems
func KillMultipleProcessGroups(cmds map[string]*exec.Cmd, sig os.Signal) error {
	var lastErr error
	for _, cmd := range cmds {
		if cmd != nil && cmd.Process != nil {
			if err := killProcessTree(uint32(cmd.Process.Pid)); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// killProcessTree kills a process and its subprocess tree on Windows
func killProcessTree(pid uint32) error {
	var entry struct {
		Size              uint32
		CntUsage          uint32
		ProcessID         uint32
		DefaultHeapID     uintptr
		ModuleID          uint32
		Threads           uint32
		ParentProcessID   uint32
		PriorityClassBase int32
		Flags             uint32
		ExeFile           [windows.MAX_PATH]uint16
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry.Size = uint32(unsafe.Sizeof(entry))

	// Find first process
	if err := windows.Process32First(snapshot, (*windows.ProcessEntry32)(unsafe.Pointer(&entry))); err != nil {
		return err
	}

	// Iterate all processes
	for {
		if entry.ParentProcessID == pid {
			// Recursively kill children first
			killProcessTree(entry.ProcessID)
		}

		err = windows.Process32Next(snapshot, (*windows.ProcessEntry32)(unsafe.Pointer(&entry)))
		if err != nil {
			break
		}
	}

	// Finally, kill this process
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err == nil {
		defer windows.CloseHandle(h)
		windows.TerminateProcess(h, 1)
	}

	return nil
}
