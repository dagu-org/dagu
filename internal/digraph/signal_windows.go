//go:build windows
// +build windows

package digraph

import (
    "syscall"
)

// GetSignalNum returns the signal number for the given signal name
func GetSignalNum(sig string) int {
    switch sig {
    case "SIGTERM":
        return int(syscall.SIGTERM)
    case "SIGINT":
        return int(syscall.SIGINT)
    case "SIGKILL":
        return int(syscall.SIGKILL)
    case "SIGHUP":
        return 1 // Windows doesn't have SIGHUP, use 1
    case "SIGUSR1":
        return 10 // Map to Windows equivalent
    case "SIGUSR2":
        return 12 // Map to Windows equivalent
    default:
        return int(syscall.SIGTERM)
    }
}

func getSignalNum(sig string) int {
    return GetSignalNum(sig)
}
