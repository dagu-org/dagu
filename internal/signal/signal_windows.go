//go:build windows
// +build windows

package signal

import (
	"syscall"
)

var signalMap = map[syscall.Signal]signalInfo{
	syscall.SIGABRT: {"SIGABRT", true, syscall.SIGABRT},
	syscall.SIGFPE:  {"SIGFPE", true, syscall.SIGFPE},
	syscall.SIGILL:  {"SIGILL", true, syscall.SIGILL},
	syscall.SIGKILL: {"SIGKILL", true, syscall.SIGKILL},
	syscall.SIGHUP:  {"SIGHUP", true, syscall.SIGHUP},
	syscall.SIGINT:  {"SIGINT", true, syscall.SIGINT},
	syscall.SIGSEGV: {"SIGSEGV", true, syscall.SIGSEGV},
	syscall.SIGTERM: {"SIGTERM", true, syscall.SIGTERM},
}

func signalName(sig syscall.Signal) string {
	if info, ok := signalMap[sig]; ok {
		return info.name
	}
	return ""
}

func isTerminationSignalInternal(sig syscall.Signal) bool {
	if info, ok := signalMap[sig]; ok {
		return info.isTermination
	}
	return false
}

// getSignalNum returns the signal number for the given signal name
func getSignalNum(sig string) int {
	for s, info := range signalMap {
		if info.name == sig {
			return int(s)
		}
	}
	// Fallback mapping for common signals on Windows
	switch sig {
	case "SIGUSR1":
		return 10 // Map to Windows equivalent
	case "SIGUSR2":
		return 12 // Map to Windows equivalent
	default:
		return int(syscall.SIGTERM)
	}
}
