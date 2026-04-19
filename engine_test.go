// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dagu_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu"
)

func TestEngineRunFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	home := t.TempDir()
	dagFile := filepath.Join(home, "embedded-file.yaml")
	writeDAG(t, dagFile, `
name: embedded-file
steps:
  - name: check-param
    command: test "$FOO" = "bar"
`)

	engine, err := dagu.New(ctx, dagu.Options{HomeDir: home})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := engine.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	run, err := engine.RunFile(ctx, dagFile, dagu.WithParams(map[string]string{"FOO": "bar"}))
	if err != nil {
		t.Fatalf("RunFile() error = %v", err)
	}
	status, err := run.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status.Status)
	}
	if status.Name != "embedded-file" {
		t.Fatalf("name = %q, want embedded-file", status.Name)
	}
	if status.RunID == "" {
		t.Fatal("run ID is empty")
	}
}

func TestEngineRunYAML(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	originalWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	engine, err := dagu.New(ctx, dagu.Options{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := engine.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	run, err := engine.RunYAML(ctx, []byte(`
name: embedded-yaml
steps:
  - name: hello
    command: echo hello
`))
	if err != nil {
		t.Fatalf("RunYAML() error = %v", err)
	}
	status, err := run.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status.Status)
	}
	if status.Name != "embedded-yaml" {
		t.Fatalf("name = %q, want embedded-yaml", status.Name)
	}
	currentWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after run error = %v", err)
	}
	if currentWorkingDir != originalWorkingDir {
		t.Fatalf("working directory = %q, want %q", currentWorkingDir, originalWorkingDir)
	}
}

func TestEngineDistributedRequiresCoordinator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	engine, err := dagu.New(ctx, dagu.Options{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := engine.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	_, err = engine.RunYAML(ctx, []byte(`
name: embedded-distributed
steps:
  - name: hello
    command: echo hello
`), dagu.WithMode(dagu.ExecutionModeDistributed))
	if err == nil {
		t.Fatal("RunYAML() error = nil, want coordinator error")
	}
	if !strings.Contains(err.Error(), "coordinator") {
		t.Fatalf("RunYAML() error = %v, want coordinator error", err)
	}
}

func writeDAG(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write DAG: %v", err)
	}
}
