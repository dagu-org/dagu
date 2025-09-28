package signal

import (
	"syscall"
)

var nameToSignal = map[string]syscall.Signal{}

func init() {
	for sig, info := range signalMap {
		nameToSignal[info.name] = sig
	}
}

// GetSignalName returns the signal name for the given signal number
func GetSignalName(sig syscall.Signal) string {
	if name := signalName(sig); name != "" {
		return name
	}
	return ""
}

// IsTerminationSignal checks if the given signal is a termination signal
func IsTerminationSignal(sig syscall.Signal) bool {
	return isTerminationSignalInternal(sig)
}

type signalInfo struct {
	name          string
	isTermination bool
	number        syscall.Signal
}

// GetSignalNum returns the signal number for the given signal name
func GetSignalNum(sig string) int {
	return getSignalNum(sig)
}
