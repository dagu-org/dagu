// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	windowsFileRetryAttempts    = 12
	windowsFileRetryInitialWait = 10 * time.Millisecond
	windowsFileRetryMaxWait     = 100 * time.Millisecond
)

// ReadFileWithRetry retries transient Windows sharing violations while reading
// files that may be momentarily held by another process.
func ReadFileWithRetry(path string) ([]byte, error) {
	var data []byte
	err := retryWindowsFileOp(func() error {
		readData, err := os.ReadFile(path) //nolint:gosec // caller controls internal path
		if err != nil {
			return err
		}
		data = readData
		return nil
	})
	return data, err
}

// RemoveWithRetry retries transient Windows sharing violations while deleting a file.
func RemoveWithRetry(path string) error {
	return retryWindowsFileOp(func() error {
		return os.Remove(path)
	})
}

// RenameWithRetry retries transient Windows sharing violations while renaming a file.
func RenameWithRetry(oldPath, newPath string) error {
	return retryWindowsFileOp(func() error {
		return os.Rename(oldPath, newPath)
	})
}

// ReplaceFileWithRetry replaces target with source, retrying transient Windows
// sharing violations that can happen while another process is still releasing
// the target file handle.
func ReplaceFileWithRetry(source, target string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(source, target)
	}

	return retryWindowsFileOp(func() error {
		info, err := os.Stat(target)
		switch {
		case err == nil:
			if info.IsDir() {
				return fmt.Errorf("target path is a directory: %s", target)
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
		case !os.IsNotExist(err):
			return err
		}
		return os.Rename(source, target)
	})
}

func retryWindowsFileOp(op func() error) error {
	err := op()
	if err == nil || !isTransientWindowsFileError(err) {
		return err
	}

	wait := windowsFileRetryInitialWait
	for range windowsFileRetryAttempts {
		time.Sleep(wait)
		err = op()
		if err == nil || !isTransientWindowsFileError(err) {
			return err
		}
		wait *= 2
		if wait > windowsFileRetryMaxWait {
			wait = windowsFileRetryMaxWait
		}
	}

	return err
}

func isTransientWindowsFileError(err error) bool {
	if runtime.GOOS != "windows" || err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "used by another process") ||
		strings.Contains(msg, "cannot access the file") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "sharing violation")
}
