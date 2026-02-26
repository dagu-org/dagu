//go:build linux

package filedoc

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime returns the file's ctime on Linux as a fallback for birth time.
func fileCreationTime(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	}
	return info.ModTime()
}
