// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import "bufio"

const (
	ScannerInitialBufferSize = 64 * 1024
	ScannerMaxTokenSize      = 10 * 1024 * 1024
)

func ConfigureScanner(scanner *bufio.Scanner) {
	if scanner == nil {
		return
	}
	scanner.Buffer(make([]byte, ScannerInitialBufferSize), ScannerMaxTokenSize)
}
