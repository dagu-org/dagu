// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec066_rclone holds black-box conformance tests for Spec 066:
// Rclone Action (rclone@v1).
//
// Like node-script@v1 (spec 060), python-script@v1 (spec 061), dbt@v1
// (spec 062), ffmpeg@v1 (spec 064), and github-cli@v1 (spec 065),
// rclone@v1 is a remote official action: referencing it clones
// https://github.com/dagucloud/rclone at the given tag and provisions its
// own pinned rclone and Node.js toolchain (via the project's aqua-based
// tool manager) into the isolated $HOME each test run gets, on first use,
// then runs the real rclone binary it installs against a real local
// directory (no external storage backend is needed to exercise copy/sync/
// list). Steps that reach the wrapper are chained with depends +
// continue_on: failed (set on the failing step itself, not its dependent --
// continue_on on a step whose predecessor failed at schema-validation time
// does not let it proceed, but does when the predecessor failed at
// runtime), to avoid concurrent cold invocations racing GitHub's
// aqua-registry API (see spec 064).
package spec066_rclone_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func rcloneSrcDstEnv(dagu *harness.Runner) []string {
	dagu.WriteFile("src/a.txt", "hello\n")
	dagu.WriteFile("src/b.txt", "world\n")
	dagu.Mkdir("dst")
	return []string{
		"HOST_SRC_DIR=" + dagu.ProjectPath("src"),
		"HOST_DST_DIR=" + dagu.ProjectPath("dst"),
	}
}

func TestRcloneLive(t *testing.T) {
	// No subtest below uses t.Parallel(): each calls t.Setenv to raise the
	// harness's per-command timeout defensively, and t.Setenv itself
	// forbids t.Parallel() in the same test.
	t.Run("rclone lists, dry-run previews, and a real copy actually copies files", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		env := rcloneSrcDstEnv(dagu)
		result := dagu.RunWithEnv(env, "start", "happy_path.yaml")
		result.ExpectExitCode(0)

		var listFiles struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Command  string `json:"command"`
			Stdout   string `json:"stdout"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &listFiles))
		require.True(t, listFiles.OK)
		require.Zero(t, listFiles.ExitCode)
		require.Equal(t, "lsf", listFiles.Command)
		require.Contains(t, listFiles.Stdout, "a.txt")
		require.Contains(t, listFiles.Stdout, "b.txt")

		var syncDryRun struct {
			OK     bool   `json:"ok"`
			Stderr string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &syncDryRun))
		require.True(t, syncDryRun.OK)
		require.Contains(t, syncDryRun.Stderr, "Skipped copy as --dry-run is set",
			"dryRun: true must report what it would have copied without actually copying it")

		var copyReal struct {
			OK bool `json:"ok"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &copyReal))
		require.True(t, copyReal.OK)
		copied, err := os.ReadFile(filepath.Join(dagu.ProjectPath("dst"), "a.txt")) // #nosec G304 -- test's own known fixture output.
		require.NoError(t, err, "allowDestructive: true should have actually copied a.txt")
		require.Equal(t, "hello\n", string(copied))
	})

	t.Run("a missing path, a destructive command without allowDestructive, and a real rclone failure all populate a string error", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		env := rcloneSrcDstEnv(dagu)
		result := dagu.RunWithEnv(env, "start", "error_scenarios.yaml")
		result.ExpectNonZeroExitCode()

		// with.command missing, an unsupported with.command, and an unknown
		// with: field are all caught by the action's own input schema
		// before the wrapper ever runs, so none of these three steps has a
		// stdout log of its own to read. rclone@v1 has no with.timeoutSeconds
		// field at all (unlike ffmpeg@v1/dbt@v1/github-cli@v1) -- the
		// schema's additionalProperties: false rejects it outright.
		require.Contains(t, result.Stdout(), `missing properties: ["command"]`)
		require.Contains(t, result.Stdout(), "does not equal any of")
		require.Contains(t, result.Stdout(), "unexpected additional properties")

		// with.source/with.destination being required by the requested
		// command, and a destructive command needing allowDestructive or
		// dryRun, are all wrapper-level checks that go beyond what the
		// input schema expresses (the schema cannot make a field's
		// requiredness conditional on with.command's value) -- reachable
		// through schema-valid input, unlike dbt@v1's near-unreachable
		// error path. All three set exitCode: 2 and a plain string error,
		// without ever invoking rclone.
		var missingSource struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Error    string `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &missingSource))
		require.False(t, missingSource.OK)
		require.Equal(t, 2, missingSource.ExitCode)
		require.Contains(t, missingSource.Error, "source is required")

		var missingDestination struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &missingDestination))
		require.False(t, missingDestination.OK)
		require.Contains(t, missingDestination.Error, "destination is required")

		var destructiveWithoutAllow struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &destructiveWithoutAllow))
		require.False(t, destructiveWithoutAllow.OK)
		require.Contains(t, destructiveWithoutAllow.Error, "allowDestructive")

		// Unlike ffmpeg@v1/dbt@v1/github-cli@v1, rclone@v1 populates a
		// string error field even for a real rclone process failure (not
		// just a wrapper-level validation error) -- here, a nonexistent
		// source directory.
		var realFailure struct {
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exitCode"`
			Error    string `json:"error"`
			Stderr   string `json:"stderr"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &realFailure))
		require.False(t, realFailure.OK)
		require.Equal(t, 3, realFailure.ExitCode)
		require.Contains(t, realFailure.Error, "rclone exited with code 3")
		require.Contains(t, realFailure.Stderr, "directory not found")
	})

	// Like node-script@v1, python-script@v1, dbt@v1, ffmpeg@v1, and
	// github-cli@v1, a later step reads a result field as
	// ${<step id>.outputs.<path>} -- a bare-step-id reference, not
	// ${steps.<step id>.outputs.<name>} -- confirming this is a property
	// of remote action:-type steps generally, not specific to one action.
	t.Run("a later step reads a result field as ${<step id>.outputs.<name>}, but not via steps.<id>.outputs.<name>", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		env := rcloneSrcDstEnv(dagu)
		result := dagu.RunWithEnv(env, "start", "downstream_reference.yaml")
		result.ExpectNonZeroExitCode()
		result.ExpectStderrContains("bad substitution")

		require.Equal(t, "bare exit code is 0\n", stepStdout(t, result.Stdout(), 1))
	})
}

// TestRcloneValidation proves that dagu validate never resolves a remote
// action reference (which would require network access): a rclone@v1 step
// passes validate regardless of its with: content, and even with.command --
// required by the action's own inputs schema -- is enforced only once the
// step actually runs and that schema is fetched. The one thing validate
// does check locally is the action reference's own syntax.
func TestRcloneValidation(t *testing.T) {
	t.Parallel()

	t.Run("a rclone@v1 step with no with.command passes validate", func(t *testing.T) {
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
		result.ExpectStderrContains(`unknown action "rclone"`)
	})
}
