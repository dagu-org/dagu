// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package archive

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func globMatch(pattern, value string) bool {
	if pattern == "" {
		return false
	}
	normalized := filepath.ToSlash(value)
	pattern = strings.TrimSpace(pattern)
	ok, err := doublestar.Match(pattern, normalized)
	if err != nil {
		return false
	}
	return ok
}
