//go:build unix
// +build unix

package signal

import "syscall"

// See https://pubs.opengroup.org/onlinepubs/9699919799/

var signalMap = map[syscall.Signal]signalInfo{
	syscall.SIGABRT:   {"SIGABRT", true, syscall.SIGABRT},     // A - Process abort signal
	syscall.SIGALRM:   {"SIGALRM", true, syscall.SIGALRM},     // T - Alarm clock
	syscall.SIGBUS:    {"SIGBUS", true, syscall.SIGBUS},       // A - Access to undefined portion of memory object
	syscall.SIGCHLD:   {"SIGCHLD", false, syscall.SIGCHLD},    // I - Child process terminated, stopped, or continued
	syscall.SIGCONT:   {"SIGCONT", false, syscall.SIGCONT},    // C - Continue executing, if stopped
	syscall.SIGFPE:    {"SIGFPE", true, syscall.SIGFPE},       // A - Erroneous arithmetic operation
	syscall.SIGHUP:    {"SIGHUP", true, syscall.SIGHUP},       // T - Hangup
	syscall.SIGILL:    {"SIGILL", true, syscall.SIGILL},       // A - Illegal instruction
	syscall.SIGINT:    {"SIGINT", true, syscall.SIGINT},       // T - Terminal interrupt signal
	syscall.SIGIO:     {"SIGIO", true, syscall.SIGIO},         // T - I/O possible (similar to SIGPOLL)
	syscall.SIGKILL:   {"SIGKILL", true, syscall.SIGKILL},     // T - Kill (cannot be caught or ignored)
	syscall.SIGPIPE:   {"SIGPIPE", true, syscall.SIGPIPE},     // T - Write on pipe with no one to read it
	syscall.SIGPROF:   {"SIGPROF", true, syscall.SIGPROF},     // T - Profiling timer expired
	syscall.SIGQUIT:   {"SIGQUIT", true, syscall.SIGQUIT},     // A - Terminal quit signal
	syscall.SIGSEGV:   {"SIGSEGV", true, syscall.SIGSEGV},     // A - Invalid memory reference
	syscall.SIGSTOP:   {"SIGSTOP", false, syscall.SIGSTOP},    // S - Stop executing (cannot be caught or ignored)
	syscall.SIGSYS:    {"SIGSYS", true, syscall.SIGSYS},       // A - Bad system call
	syscall.SIGTERM:   {"SIGTERM", true, syscall.SIGTERM},     // T - Termination signal
	syscall.SIGTRAP:   {"SIGTRAP", true, syscall.SIGTRAP},     // A - Trace/breakpoint trap
	syscall.SIGTSTP:   {"SIGTSTP", false, syscall.SIGTSTP},    // S - Terminal stop signal
	syscall.SIGTTIN:   {"SIGTTIN", false, syscall.SIGTTIN},    // S - Background process attempting read
	syscall.SIGTTOU:   {"SIGTTOU", false, syscall.SIGTTOU},    // S - Background process attempting write
	syscall.SIGURG:    {"SIGURG", false, syscall.SIGURG},      // I - High bandwidth data available at socket
	syscall.SIGUSR1:   {"SIGUSR1", true, syscall.SIGUSR1},     // T - User-defined signal 1
	syscall.SIGUSR2:   {"SIGUSR2", true, syscall.SIGUSR2},     // T - User-defined signal 2
	syscall.SIGVTALRM: {"SIGVTALRM", true, syscall.SIGVTALRM}, // T - Virtual timer expired
	syscall.SIGWINCH:  {"SIGWINCH", false, syscall.SIGWINCH},  // I - Window size change (not in POSIX table)
	syscall.SIGXCPU:   {"SIGXCPU", true, syscall.SIGXCPU},     // A - CPU time limit exceeded
	syscall.SIGXFSZ:   {"SIGXFSZ", true, syscall.SIGXFSZ},     // A - File size limit exceeded
}

func isTerminationSignalInternal(sig syscall.Signal) bool {
	if sigInfo, ok := signalMap[sig]; ok {
		return sigInfo.isTermination
	}
	return false
}
