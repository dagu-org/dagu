// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package cmdutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetShellCommand_WindowsPrefersNativeShellsOverSHELL(t *testing.T) {
	originalShell := os.Getenv("SHELL")
	originalDAGUShell := os.Getenv("DAGU_DEFAULT_SHELL")
	defer func() {
		if originalShell != "" {
			_ = os.Setenv("SHELL", originalShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
		if originalDAGUShell != "" {
			_ = os.Setenv("DAGU_DEFAULT_SHELL", originalDAGUShell)
		} else {
			_ = os.Unsetenv("DAGU_DEFAULT_SHELL")
		}
	}()

	_ = os.Unsetenv("DAGU_DEFAULT_SHELL")
	_ = os.Setenv("SHELL", `/usr/bin/bash`)

	result := GetShellCommand("")
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(result)), ".exe")

	assert.NotEmpty(t, result)
	assert.Contains(t, []string{"powershell", "pwsh", "cmd"}, base)
	assert.NotEqual(t, "bash", base)
}
