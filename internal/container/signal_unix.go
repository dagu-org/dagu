//go:build unix

package container

import "syscall"

var signalMap = map[syscall.Signal]string{
	syscall.SIGABRT: "SIGABRT",
	syscall.SIGALRM: "SIGALRM",
	syscall.SIGBUS:  "SIGBUS",
	syscall.SIGFPE:  "SIGFPE",
	syscall.SIGHUP:  "SIGHUP",
	syscall.SIGILL:  "SIGILL",
	syscall.SIGINT:  "SIGINT",
	syscall.SIGKILL: "SIGKILL",
	syscall.SIGPIPE: "SIGPIPE",
	syscall.SIGQUIT: "SIGQUIT",
	syscall.SIGSEGV: "SIGSEGV",
	syscall.SIGTERM: "SIGTERM",
	syscall.SIGTRAP: "SIGTRAP",
}

func signalName(sig syscall.Signal) string {
	if name, ok := signalMap[sig]; ok {
		return name
	}
	return ""
}
