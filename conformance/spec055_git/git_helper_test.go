// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec055_git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// originRepo describes a local git repository this test suite builds from
// scratch (via the real git binary) to act as a checkout source, so tests
// run hermetically against a real repository without any network access.
type originRepo struct {
	Path        string
	FirstCommit string // tagged "first"; file.txt contains "v1"
	HeadCommit  string // main's HEAD; file.txt contains "v2"
}

// setupOriginRepo creates a two-commit git repository in a fresh temp
// directory: the first commit (tagged "first") writes file.txt="v1", the
// second (left as HEAD) writes file.txt="v2".
func setupOriginRepo(t *testing.T) originRepo {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "conformance@example.com")
	runGit(t, dir, "config", "user.name", "Conformance Test")

	writeAndCommit(t, dir, "v1", "commit1")
	first := runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "tag", "first")

	writeAndCommit(t, dir, "v2", "commit2")
	head := runGit(t, dir, "rev-parse", "HEAD")

	return originRepo{Path: dir, FirstCommit: first, HeadCommit: head}
}

func writeAndCommit(t *testing.T, dir, content, message string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content), 0o600))
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...) // #nosec G204 -- fixed args/dir in test setup, not user input.
	cmd.Dir = dir
	// Ignore host Git configuration so fixture commits need no hooks or signing.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.Output()
	require.NoErrorf(t, err, "git %s: %v", strings.Join(args, " "), err)
	return strings.TrimSpace(string(out))
}
