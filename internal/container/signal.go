package container

import "syscall"

func GetSignalName(sig syscall.Signal) string {
	if name := signalName(sig); name != "" {
		return name
	}
	return ""
}
