// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// SupportsPOSIXPermissionBits reports whether exact Unix permission-bit
// assertions are meaningful on the current platform.
func SupportsPOSIXPermissionBits() bool {
	return runtime.GOOS != "windows"
}

// SkipIfPOSIXPermissionErrorsUnsupported skips tests that rely on chmod-based
// permission failures, which are not reliable on Windows or as root.
func SkipIfPOSIXPermissionErrorsUnsupported(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce POSIX chmod permission failures")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
}

// BlockPathWithFile ensures path is occupied by a regular file.
func BlockPathWithFile(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("write blocker file %s: %v", path, err)
	}
}

// BlockPathWithDirectory ensures path is occupied by a directory.
func BlockPathWithDirectory(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove %s: %v", path, err)
	}
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir blocker directory %s: %v", path, err)
	}
}
