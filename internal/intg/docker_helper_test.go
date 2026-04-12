// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func requireDockerDaemon(t *testing.T) {
	t.Helper()

	dockerClient, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("Skipping Docker-backed integration test: failed to create docker client: %v", err)
	}
	defer func() { _ = dockerClient.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := dockerClient.Info(ctx, client.InfoOptions{}); err != nil {
		t.Skipf("Skipping Docker-backed integration test: docker daemon unavailable: %v", err)
	}
}

func requireDockerClient(t *testing.T) *client.Client {
	t.Helper()

	dockerClient, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("Skipping Docker-backed integration test: failed to create docker client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := dockerClient.Info(ctx, client.InfoOptions{}); err != nil {
		_ = dockerClient.Close()
		t.Skipf("Skipping Docker-backed integration test: docker daemon unavailable: %v", err)
	}

	t.Cleanup(func() { _ = dockerClient.Close() })
	return dockerClient
}
