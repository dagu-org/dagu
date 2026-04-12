// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func canonicalTestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func requireLinuxContainerRuntime(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "windows" {
		return
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Skipf("Docker integration tests require a Linux container runtime: %v", err)
	}
	defer func() { _ = dockerClient.Close() }()

	info, err := dockerClient.Info(context.Background(), client.InfoOptions{})
	if err != nil {
		t.Skipf("Docker integration tests require a Linux container runtime: %v", err)
	}

	if info.Info.OSType != "linux" {
		t.Skipf("Docker integration tests require a Linux container runtime, got %q", info.Info.OSType)
	}
}
