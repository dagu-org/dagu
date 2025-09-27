//go:build windows

package container

import "syscall"

var signalMap = map[syscall.Signal]string{
	syscall.SIGABRT: "SIGABRT",
	syscall.SIGFPE:  "SIGFPE",
	syscall.SIGILL:  "SIGILL",
	syscall.SIGINT:  "SIGINT",
	syscall.SIGSEGV: "SIGSEGV",
	syscall.SIGTERM: "SIGTERM",
}

func signalName(sig syscall.Signal) string {
	if name, ok := signalMap[sig]; ok {
		return name
	}
	return ""
}
