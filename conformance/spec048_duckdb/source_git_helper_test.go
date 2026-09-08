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
	"github.com/dagucloud/dagu/v2/internal/cmn/cmdutil"
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
var gitEnv = []string{
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_CONFIG_GLOBAL=" + os.DevNull,
	// Git must support nested action-cache paths on Windows.
	"GIT_CONFIG_COUNT=1",
	"GIT_CONFIG_KEY_0=core.longpaths",
	"GIT_CONFIG_VALUE_0=true",
}

// startGitActionServer serves the local action fixture over Git transport.
func startGitActionServer(t *testing.T) int {
	t.Helper()

	basePath := t.TempDir()
	repoDir := filepath.Join(basePath, "echo-action")
	require.NoError(t, os.MkdirAll(repoDir, 0o750))

	runGit(t, repoDir, "init", "-q", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "conformance@example.com")
	runGit(t, repoDir, "config", "user.name", "Conformance Test")

	require.NoError(t, os.CopyFS(repoDir, os.DirFS("testdata/echo_action")))

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
	proc, err := cmdutil.StartManagedProcess(cmd)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = proc.Stop(cmdutil.StopRequest{
			Intent: cmdutil.ForceTermination(),
			Reason: cmdutil.StopReasonShutdown,
		})
		_ = proc.Wait()
		_ = proc.Release()
	})

	waitForPort(t, port)
	return port
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
