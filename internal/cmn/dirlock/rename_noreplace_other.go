// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux && !darwin

package dirlock

import "os"

// renameNoReplace renames oldPath to newPath. On Windows the rename never
// replaces an existing directory. Elsewhere os.Rename rejects an existing
// directory only after checking for it, so an empty directory created in
// between can still be replaced.
func renameNoReplace(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
