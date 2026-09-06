// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec065_github_cli holds black-box conformance tests for Spec
// 065: GitHub CLI Action (github-cli@v1).
//
// Like node-script@v1 (spec 060), python-script@v1 (spec 061), dbt@v1
// (spec 062), and ffmpeg@v1 (spec 064), github-cli@v1 is a remote official
// action: referencing it clones https://github.com/dagucloud/github-cli at
// the given tag and provisions its own pinned gh CLI and Node.js toolchain
// (via the project's aqua-based tool manager) into the isolated $HOME each
// test run gets, on first use, then runs the real gh binary it installs.
// This environment has no GitHub credentials configured, so every gh
// subcommand that needs auth (almost all of them, including read-only ones
// like "repo view") fails with a real, deterministic gh error -- this
// package treats that as ground truth rather than a limitation: it proves
// with.env/with.host/with.repo reach the real gh process by observing how
// they change which real error gh reports, and reserves a successful
// (ok: true) run for "gh --version", the one subcommand that needs neither
// auth nor network. Steps that reach the wrapper are chained with
// depends+continue_on: failed rather than left independent, to avoid several
// concurrent cold invocations racing GitHub's aqua-registry API and hitting
// its rate limit (this previously caused spec 064's suite to intermittently
// stall on 403 responses).
package spec065_github_cli_test

import (
	"encoding/json"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestGithubCliLive(t *testing.T) {
	// No subtest below uses t.Parallel(): each calls t.Setenv to raise the
	// harness's per-command timeout defensively, and t.Setenv itself
	// forbids t.Parallel() in the same test.
	t.Run("gh runs for real, and with.workdir/repo/host/env are all accepted without breaking a successful run", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		dagu.Mkdir("workdir")
		env := []string{"HOST_WORK_DIR=" + dagu.ProjectPath("workdir")}
		result := dagu.RunWithEnv(env, "start", "happy_path.yaml")
		result.ExpectExitCode(0)

		var version struct {
			OK        bool   `json:"ok"`
			ExitCode  int    `json:"exitCode"`
			GhVersion string `json:"ghVersion"`
			Stdout    string `json:"stdout"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &version))
		require.True(t, version.OK)
		require.Zero(t, version.ExitCode)
		require.Contains(t, version.GhVersion, "gh version 2.92.0")
		require.Contains(t, version.Stdout, "gh version 2.92.0")

		var withFields struct {
			OK       bool `json:"ok"`
			ExitCode int  `json:"exitCode"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &withFields))
		require.True(t, withFields.OK, "with.workdir/repo/host/env should not interfere with a command that ignores all of them")
		require.Zero(t, withFields.ExitCode)
	})

	t.Run("a real gh failure (no auth, a bad token, an unreachable host) reports through exitCode/stderr, and a blackholed host times out", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		result := dagu.Run("start", "error_scenarios.yaml")
		result.ExpectNonZeroExitCode()

		// with.args missing, with.args an empty array, and with.timeoutSeconds
		// out of range are all caught by the action's own input schema
		// before the wrapper ever runs, so none of these three steps has a
		// stdout log of its own to read.
		require.Contains(t, result.Stdout(), `missing properties: ["args"]`)
		require.Contains(t, result.Stdout(), "less than 1")
		require.Contains(t, result.Stdout(), "greater than")

		// This sandbox has no gh auth configured at all, so any command
		// needing it (almost all of them) fails locally and immediately
		// with gh's own "not authenticated" error -- a real, deterministic
		// gh outcome, not a wrapper-level validation error.
		var noAuth struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Stderr   string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &noAuth))
		require.False(t, noAuth.OK)
		require.Equal(t, 4, noAuth.ExitCode)
		require.Contains(t, noAuth.Stderr, "gh auth login")

		// with.env reaches the real gh process: a bogus GH_TOKEN clears
		// gh's own local auth gate (a token is present) and gh attempts a
		// real GitHub API call, which then fails with a real HTTP 401 --
		// proof the token value itself made it to the process.
		var badToken struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Stderr   string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &badToken))
		require.False(t, badToken.OK)
		require.Equal(t, 1, badToken.ExitCode)
		require.Contains(t, badToken.Stderr, "Bad credentials")

		// with.host reaches the real gh process too: gh attempts to
		// resolve/connect to the given host instead of github.com.
		var badHost struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Stderr   string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &badHost))
		require.False(t, badHost.OK)
		require.Equal(t, 1, badHost.ExitCode)
		require.Contains(t, badHost.Stderr, "gh.invalid.example.test")

		// with.timeoutSeconds: a host address that never responds (a
		// TEST-NET-1 address, RFC 5737) blocks the real network connection
		// well past the 2-second timeout, so the wrapper sends SIGTERM.
		// The wrapper forces exitCode to 124 on any timeout, unlike
		// ffmpeg@v1, which reports the real process's own signal-exit
		// code. gh does not trap SIGTERM, so it is actually killed by the
		// (unhandled) signal, and the wrapper synthesizes a "terminated by
		// <signal>" stderr message since gh itself produced none.
		var timedOut struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			TimedOut bool   `json:"timedOut"`
			Stderr   string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &timedOut))
		require.False(t, timedOut.OK)
		require.Equal(t, 124, timedOut.ExitCode)
		require.True(t, timedOut.TimedOut)
		require.Contains(t, timedOut.Stderr, "terminated by SIGTERM")
	})

	// Like node-script@v1, python-script@v1, dbt@v1, and ffmpeg@v1, a later
	// step reads a result field as ${<step id>.outputs.<name>} -- a
	// bare-step-id reference, not ${steps.<step id>.outputs.<name>} --
	// confirming this is a property of remote action:-type steps generally,
	// not specific to one action.
	t.Run("a later step reads a result field as ${<step id>.outputs.<name>}, but not via steps.<id>.outputs.<name>", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		result := dagu.Run("start", "downstream_reference.yaml")
		result.ExpectNonZeroExitCode()
		result.ExpectStderrContains("bad substitution")

		require.Equal(t, "bare exit code is 0\n", stepStdout(t, result.Stdout(), 1))
	})
}

// TestGithubCliValidation proves that dagu validate never resolves a remote
// action reference (which would require network access): a github-cli@v1
// step passes validate regardless of its with: content, and even with.args
// -- required by the action's own inputs schema -- is enforced only once
// the step actually runs and that schema is fetched. The one thing
// validate does check locally is the action reference's own syntax.
func TestGithubCliValidation(t *testing.T) {
	t.Parallel()

	t.Run("a github-cli@v1 step with no with.args passes validate", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.Run("validate", "error_scenarios.yaml")
		result.ExpectExitCode(0)
	})

	t.Run("an action reference missing its required @version suffix fails validate", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.Run("validate", "no_version_suffix.yaml")
		result.ExpectNonZeroExitCode()
		result.ExpectStderrContains(`unknown action "github-cli"`)
	})
}
