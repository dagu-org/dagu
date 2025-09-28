package signal

import (
	"os"
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

// IsTerminationSignalOS checks if the given os.Signal is a termination signal
func IsTerminationSignalOS(sis os.Signal) bool {
	sig, ok := sis.(syscall.Signal)
	if !ok {
		return false
	}
	return isTerminationSignalInternal(sig)
}

// IsTerminationSignal checks if the given signal is a termination signal
func IsTerminationSignal(sig syscall.Signal) bool {
	return isTerminationSignalInternal(sig)
}

// GetSignalNum returns the signal number for the given signal name
func GetSignalNum(sig string) int {
	return getSignalNum(sig)
}

type signalInfo struct {
	name          string
	isTermination bool
	number        syscall.Signal
}
