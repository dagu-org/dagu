// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package fileutil

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
