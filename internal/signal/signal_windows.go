//go:build windows
// +build windows

package signal

import (
	"syscall"
)

var signalMap = map[syscall.Signal]signalInfo{
	syscall.SIGABRT:    {"SIGABRT", true, syscall.SIGABRT},
	syscall.SIGFPE:     {"SIGFPE", true, syscall.SIGFPE},
	syscall.SIGILL:     {"SIGILL", true, syscall.SIGILL},
	syscall.SIGKILL:    {"SIGKILL", true, syscall.SIGKILL},
	syscall.SIGHUP:     {"SIGHUP", true, syscall.SIGHUP},
	syscall.SIGINT:     {"SIGINT", true, syscall.SIGINT},
	syscall.SIGSEGV:    {"SIGSEGV", true, syscall.SIGSEGV},
	syscall.SIGTERM:    {"SIGTERM", true, syscall.SIGTERM},
	syscall.Signal(10): {"SIGUSR1", true, syscall.Signal(10)}, // Map to Windows equivalent
	syscall.Signal(12): {"SIGUSR2", true, syscall.Signal(12)}, // Map to Windows equivalent
}

func isTerminationSignalInternal(sig syscall.Signal) bool {
	if info, ok := signalMap[sig]; ok {
		return info.isTermination
	}
	return false
}
