// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package filegithubdispatch

// Windows does not support syncing directory handles through os.File.Sync.
// Keep rename durability best-effort there while preserving strict syncing on
// platforms that support directory fsync.
func syncTrackerDir(string) error {
	return nil
}
