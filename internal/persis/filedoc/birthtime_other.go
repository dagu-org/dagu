//go:build !darwin && !linux

package filedoc

import (
	"os"
	"time"
)

// fileCreationTime falls back to ModTime on unsupported platforms.
func fileCreationTime(info os.FileInfo) time.Time {
	return info.ModTime()
}
