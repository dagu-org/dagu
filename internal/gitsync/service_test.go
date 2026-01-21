// Copyright (C) 2025 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package gitsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestService_GetStatus(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitsync-svc-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dagsDir := filepath.Join(tempDir, "dags")
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dagsDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg := &Config{
		Enabled:    true,
		Repository: "host.com/org/repo",
		Branch:     "main",
	}

	svc := NewService(cfg, dagsDir, dataDir)
	status, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.Enabled {
		t.Error("status.Enabled = false, want true")
	}
	if status.Repository != cfg.Repository {
		t.Errorf("status.Repository = %v, want %v", status.Repository, cfg.Repository)
	}
	if status.Branch != cfg.Branch {
		t.Errorf("status.Branch = %v, want %v", status.Branch, cfg.Branch)
	}
}

func TestService_PathHelpers(t *testing.T) {
	s := &serviceImpl{
		dagsDir: "/dags",
		cfg: &Config{
			Path: "subdir",
		},
	}

	// Test filePathToDAGID
	dagID := s.filePathToDAGID("subdir/my_dag.yaml")
	if dagID != "my_dag" {
		t.Errorf("filePathToDAGID = %v, want my_dag", dagID)
	}

	// Test dagIDToFilePath
	dagPath := s.dagIDToFilePath("my_dag")
	if dagPath != "/dags/my_dag.yaml" {
		t.Errorf("dagIDToFilePath = %v, want /dags/my_dag.yaml", dagPath)
	}

	// Test dagIDToRepoPath
	repoPath := s.dagIDToRepoPath("my_dag")
	if repoPath != "subdir/my_dag.yaml" {
		t.Errorf("dagIDToRepoPath = %v, want subdir/my_dag.yaml", repoPath)
	}
}
