// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package api

import "errors"

func makeNamedPipe(string) error {
	return errors.New("named pipes via mkfifo are not supported on Windows")
}
