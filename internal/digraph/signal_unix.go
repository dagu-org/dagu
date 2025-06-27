//go:build !windows
// +build !windows

package digraph

import "golang.org/x/sys/unix"

// GetSignalNum returns the signal number for the given signal name
func GetSignalNum(sig string) int {
    return int(unix.SignalNum(sig))
}

func getSignalNum(sig string) int {
    return GetSignalNum(sig)
}
