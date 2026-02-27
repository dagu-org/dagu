//go:build darwin

package filedoc

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime returns the file's birth time on macOS.
func fileCreationTime(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	}
	return info.ModTime()
}
