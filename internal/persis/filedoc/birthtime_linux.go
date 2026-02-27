//go:build linux

package filedoc

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime returns the file's ctime (inode change time) on Linux.
// Note: This is not the true birth time; Linux birth time requires statx() which
// is not available via the standard syscall package. Ctime is used as an approximation.
func fileCreationTime(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	}
	return info.ModTime()
}
