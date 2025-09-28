//go:build unix

package signal

import (
	"syscall"
	"testing"
)

func TestSignalClassification(t *testing.T) {
	// Test key termination signals (T/A actions)
	terminationSignals := []syscall.Signal{
		syscall.SIGTERM, // T - Termination signal
		syscall.SIGINT,  // T - Terminal interrupt signal
		syscall.SIGABRT, // A - Process abort signal
		syscall.SIGSEGV, // A - Invalid memory reference
	}

	// Test key non-termination signals (I/S/C actions)
	nonTerminationSignals := []syscall.Signal{
		syscall.SIGCHLD, // I - Child process terminated
		syscall.SIGSTOP, // S - Stop executing
		syscall.SIGCONT, // C - Continue executing
		syscall.SIGURG,  // I - High bandwidth data available
	}

	for _, sig := range terminationSignals {
		if !IsTerminationSignal(sig) {
			t.Errorf("IsTerminationSignal(%v) should be true", sig)
		}
		if GetSignalName(sig) == "" {
			t.Errorf("GetSignalName(%v) should not be empty", sig)
		}
	}

	for _, sig := range nonTerminationSignals {
		if IsTerminationSignal(sig) {
			t.Errorf("IsTerminationSignal(%v) should be false", sig)
		}
		if GetSignalName(sig) == "" {
			t.Errorf("GetSignalName(%v) should not be empty", sig)
		}
	}
}

func TestGetSignalNum(t *testing.T) {
	if GetSignalNum("SIGTERM") != int(syscall.SIGTERM) {
		t.Error("GetSignalNum should return correct number for SIGTERM")
	}

	if GetSignalNum("UNKNOWN") != int(syscall.SIGTERM) {
		t.Error("GetSignalNum should return SIGTERM for unknown signals")
	}
}
