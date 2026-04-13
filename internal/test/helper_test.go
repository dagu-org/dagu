// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupDoesNotMutatePerTestProcessEnv(t *testing.T) {
	t.Setenv("DAGU_HOME", "/original/dagu-home")
	t.Setenv("DAGU_CONFIG", "/original/config.yaml")
	t.Setenv("DAGU_EXECUTABLE", "/original/dagu")
	t.Setenv("SHELL", "/original/shell")

	helper := Setup(t)

	assert.Equal(t, "/original/dagu-home", os.Getenv("DAGU_HOME"))
	assert.Equal(t, "/original/config.yaml", os.Getenv("DAGU_CONFIG"))
	assert.Equal(t, helperExpectedExecutableEnvValue("/original/dagu"), os.Getenv("DAGU_EXECUTABLE"))
	assert.Equal(t, "/original/shell", os.Getenv("SHELL"))

	assert.Contains(t, helper.ChildEnv, "DAGU_HOME="+helper.tmpDir)
	assert.Contains(t, helper.ChildEnv, "DAGU_CONFIG="+helper.Config.Paths.ConfigFileUsed)
	assert.Contains(t, helper.ChildEnv, "DAGU_EXECUTABLE="+helper.Config.Paths.Executable)
	assert.Contains(t, helper.ChildEnv, "SHELL="+helper.Config.Core.DefaultShell)
	assert.Contains(t, helper.ChildEnv, "DEBUG=true")
	assert.Contains(t, helper.ChildEnv, "CI=true")
	assert.Contains(t, helper.ChildEnv, "TZ=UTC")
}

func helperExpectedExecutableEnvValue(value string) string {
	if runtime.GOOS != "windows" {
		return value
	}

	if strings.HasPrefix(value, "/") {
		drive := os.Getenv("SystemDrive")
		if drive == "" {
			drive = filepath.VolumeName(os.TempDir())
		}
		if drive != "" {
			return filepath.Join(drive+string(os.PathSeparator), filepath.FromSlash(strings.TrimPrefix(value, "/")))
		}
	}

	return filepath.FromSlash(value)
}
