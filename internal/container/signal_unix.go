//go:build unix

package container

import "syscall"

var signalMap = map[syscall.Signal]string{
	syscall.SIGABRT:   "SIGABRT",
	syscall.SIGALRM:   "SIGALRM",
	syscall.SIGBUS:    "SIGBUS",
	syscall.SIGCHLD:   "SIGCHLD",
	syscall.SIGFPE:    "SIGFPE",
	syscall.SIGHUP:    "SIGHUP",
	syscall.SIGILL:    "SIGILL",
	syscall.SIGINT:    "SIGINT",
	syscall.SIGIO:     "SIGIO",
	syscall.SIGKILL:   "SIGKILL",
	syscall.SIGPIPE:   "SIGPIPE",
	syscall.SIGQUIT:   "SIGQUIT",
	syscall.SIGSEGV:   "SIGSEGV",
	syscall.SIGTERM:   "SIGTERM",
	syscall.SIGTRAP:   "SIGTRAP",
	syscall.SIGTSTP:   "SIGTSTP",
	syscall.SIGTTIN:   "SIGTTIN",
	syscall.SIGTTOU:   "SIGTTOU",
	syscall.SIGURG:    "SIGURG",
	syscall.SIGUSR1:   "SIGUSR1",
	syscall.SIGUSR2:   "SIGUSR2",
	syscall.SIGVTALRM: "SIGVTALRM",
	syscall.SIGWINCH:  "SIGWINCH",
	syscall.SIGXCPU:   "SIGXCPU",
	syscall.SIGXFSZ:   "SIGXFSZ",
}

func signalName(sig syscall.Signal) string {
	if name, ok := signalMap[sig]; ok {
		return name
	}
	return ""
}
