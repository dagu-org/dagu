// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinHarnessProviderNamesSorted(t *testing.T) {
	assert.Equal(t, []string{"claude", "codex", "copilot", "opencode", "pi"}, BuiltinHarnessProviderNames())
}
