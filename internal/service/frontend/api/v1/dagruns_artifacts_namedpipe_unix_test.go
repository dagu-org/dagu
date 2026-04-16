// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package api

import "syscall"

func makeNamedPipe(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
