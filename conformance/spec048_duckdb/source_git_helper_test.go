// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec048_duckdb_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// stdoutLogPattern matches a per-step captured-stdout log path dagu start
// prints in its tree render, e.g. "└─stdout: /path/to/step.<ts>.<run>.out".
var stdoutLogPattern = regexp.MustCompile(`stdout: (.+)`)

// stepStdout reads the exact bytes the (0-indexed) nth step with logged
// stdout wrote, by locating its captured-output log file from dagu start's
// own tree render and reading it directly, since the tree render re-wraps
// long lines with its own indentation, which would corrupt a strict
// JSON-parse match.
func stepStdout(t *testing.T, daguStartOutput string, n int) string {
	t.Helper()

	matches := stdoutLogPattern.FindAllStringSubmatch(daguStartOutput, -1)
	require.Greaterf(t, len(matches), n, "expected at least %d stdout log paths in output:\n%s", n+1, daguStartOutput)
	path := strings.TrimSpace(matches[n][1])
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the harness's own trusted output.
	require.NoError(t, err)
	return string(data)
}

// gitEnv bypasses this machine's own global/system git config (some hosts
// rewrite git:// to https://, or otherwise interfere) the same way
// spec055_git's own git fixtures do, so the git commands this file's setup
// and dagu's own action source resolver run see a clean, predictable git
// configuration.
var gitEnv = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}

// gitActionServer serves a real echo-action bundle (the same shape as
// testdata/echo_action) over the git:// protocol via a real git daemon, so
// tests can reference it with a genuine "source:git://...@version" URL --
// a target resolveSourceBundle cannot mistake for a local directory (unlike
// a plain path or a file:// URL), forcing it through the real
// clone-over-network code path (cloneGitSource in
// internal/runtime/builtin/action/resolver.go), the same as it would for
// any non-official, custom git-hosted action.
type gitActionServer struct {
	Port int
}

// RepoURL returns the git:// URL for the named repo this server exports.
func (s gitActionServer) RepoURL(name string) string {
	return fmt.Sprintf("git://127.0.0.1:%d/%s", s.Port, name)
}

// startGitActionServer creates a one-commit, tag "v1" git repository shaped
// like an echo-action bundle (dagu-action.yaml + workflow.yaml, requiring
// with.message and echoing it back as outputs.echoed) under a fresh temp
// directory, then serves it (and anything else placed under the same base
// directory before this call returns) with a real "git daemon" process on
// a free loopback port. The daemon is killed via t.Cleanup.
func startGitActionServer(t *testing.T, repoName string) gitActionServer {
	t.Helper()

	basePath := t.TempDir()
	repoDir := filepath.Join(basePath, repoName)
	require.NoError(t, os.MkdirAll(repoDir, 0o750))

	runGit(t, repoDir, "init", "-q", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "conformance@example.com")
	runGit(t, repoDir, "config", "user.name", "Conformance Test")

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "dagu-action.yaml"), []byte(`apiVersion: v1alpha1
name: echo-action
dag: workflow.yaml
inputs:
  type: object
  additionalProperties: false
  required: [message]
  properties:
    message:
      type: string
      minLength: 1
outputs:
  type: object
  additionalProperties: false
  required: [echoed]
  properties:
    echoed:
      type: string
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "workflow.yaml"), []byte(`type: graph
params:
  type: object
  additionalProperties: false
  required: [message]
  properties:
    message:
      type: string
steps:
  - id: echo
    run: printf '%s' "$message"
    stdout:
      outputs:
        field: echoed
`), 0o600))

	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial")
	runGit(t, repoDir, "tag", "v1")

	port := harness.FreePort(t)
	// #nosec G204 -- fixed args/port in test setup, not user input.
	cmd := exec.Command("git", "daemon",
		"--reuseaddr",
		"--export-all",
		"--listen=127.0.0.1",
		fmt.Sprintf("--port=%d", port),
		"--base-path="+basePath,
		basePath,
	)
	cmd.Env = append(os.Environ(), gitEnv...)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	waitForPort(t, port)
	return gitActionServer{Port: port}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...) // #nosec G204 -- fixed args/dir in test setup, not user input.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), gitEnv...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

func waitForPort(t *testing.T, port int) {
	t.Helper()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(harness.WaitTimeout(t))
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("git daemon did not start listening on %s in time", addr)
}
