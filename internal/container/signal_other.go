//go:build !unix && !windows

package container

import "syscall"

func signalName(syscall.Signal) string {
	return ""
}
