// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package filegithubdispatch

import (
	"fmt"
	"os"
)

func syncTrackerDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open tracker dir for sync: %w", err)
	}
	defer dir.Close() //nolint:errcheck
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync tracker dir: %w", err)
	}
	return nil
}
