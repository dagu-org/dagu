// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec064_ffmpeg holds black-box conformance tests for Spec 064:
// FFmpeg Action (ffmpeg@v1).
//
// Like node-script@v1 (spec 060), python-script@v1 (spec 061), and dbt@v1
// (spec 062), ffmpeg@v1 is a remote official action: referencing it clones
// https://github.com/dagucloud/ffmpeg at the given tag and provisions its
// own pinned FFmpeg and Node.js toolchain (via the project's aqua-based
// tool manager) into the isolated $HOME each test run gets, on first use,
// then runs the real ffmpeg/ffprobe binary it installs. Unlike the
// uv/Python-based actions, that first invocation is fast (a few seconds:
// FFmpegBin and node are single prebuilt archives, not a package-resolver
// graph), so this package does not need to raise the harness's per-command
// timeout as aggressively as spec 061/062 do -- a modest bump is still
// applied defensively for slower CI runners.
package spec064_ffmpeg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestFfmpegLive(t *testing.T) {
	// No subtest below uses t.Parallel(): each calls t.Setenv to raise the
	// harness's per-command timeout defensively, and t.Setenv itself
	// forbids t.Parallel() in the same test.
	t.Run("ffmpeg and ffprobe run against a real media file, with structured args, workdir, and env", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		dagu.Mkdir("artifacts")
		dagu.Mkdir("workdir")
		env := []string{
			"HOST_OUT_DIR=" + dagu.ProjectPath("artifacts"),
			"HOST_WORK_DIR=" + dagu.ProjectPath("workdir"),
		}
		result := dagu.RunWithEnv(env, "start", "happy_path.yaml")
		result.ExpectExitCode(0)

		var convert struct {
			OK       bool     `json:"ok"`
			ExitCode int      `json:"exitCode"`
			Command  string   `json:"command"`
			Args     []string `json:"args"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &convert))
		require.True(t, convert.OK)
		require.Zero(t, convert.ExitCode)
		require.Equal(t, "ffmpeg", convert.Command)
		require.Equal(t, []string{"-hide_banner", "-nostdin", "-y"}, convert.Args[:3],
			"ffmpeg gets -hide_banner/-nostdin (both default true) and -y (overwrite: true) prepended")
		clipPath := filepath.Join(dagu.ProjectPath("artifacts"), "clip.mp4")
		info, err := os.Stat(clipPath) // #nosec G304 -- path is this test's own known fixture output.
		require.NoError(t, err, "convert should have written a real clip.mp4")
		require.Positive(t, info.Size())

		var probe struct {
			OK      bool     `json:"ok"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Stdout  string   `json:"stdout"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &probe))
		require.True(t, probe.OK)
		require.Equal(t, "ffprobe", probe.Command)
		require.Equal(t, []string{"-hide_banner", "-v", "error"}, probe.Args[:3],
			"ffprobe does not get -nostdin or -y/-n, since those only apply to ffmpeg")
		var probeReport struct {
			Streams []struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"streams"`
		}
		require.NoError(t, json.Unmarshal([]byte(probe.Stdout), &probeReport),
			"the wrapper's own stdout field should itself be ffprobe's -print_format json report")
		require.Len(t, probeReport.Streams, 1)
		require.Equal(t, 32, probeReport.Streams[0].Width)
		require.Equal(t, 32, probeReport.Streams[0].Height)

		var structured struct {
			OK   bool     `json:"ok"`
			Args []string `json:"args"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &structured))
		require.True(t, structured.OK)
		require.Equal(t, []string{
			"-n", // overwrite defaults to false regardless of hideBanner/nostdin
			"-f", "lavfi",
			"-i", "color=c=red:s=16x16:d=1",
			"-c:v", "libx264",
			"rel_out.mp4",
		}, structured.Args, "hideBanner: false and nostdin: false suppress -hide_banner/-nostdin; args as a JSON array string is taken as exact argv")
		relOutPath := filepath.Join(dagu.ProjectPath("workdir"), "rel_out.mp4")
		info, err = os.Stat(relOutPath) // #nosec G304 -- path is this test's own known fixture output.
		require.NoError(t, err, "a relative output path should resolve against with.workdir")
		require.Positive(t, info.Size())
	})

	t.Run("bad shell quoting, a malformed env line, a real ffmpeg failure, a timeout, and output truncation", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		result := dagu.Run("start", "error_scenarios.yaml")
		result.ExpectNonZeroExitCode()

		// with.args missing, an unsupported with.command, and with.env given
		// as a native object (the action's own input schema types env as a
		// string, unlike dbt@v1's with.env) are all caught by the action's
		// own input schema before the wrapper ever runs, so none of these
		// three steps has a stdout log of its own to read.
		require.Contains(t, result.Stdout(), `missing properties: ["args"]`)
		require.Contains(t, result.Stdout(), "does not equal any of")
		require.Contains(t, result.Stdout(), `validating /properties/env`)
		require.Contains(t, result.Stdout(), `has type "object"`)

		// Bad shell quoting and a malformed env line both fail inside the
		// wrapper itself (its own args/env parsing goes beyond what the
		// input schema checks), so both report a populated "error" object
		// with ok:false and exitCode:-1, unlike dbt@v1 where that path is
		// effectively unreachable.
		var badQuoting struct {
			OK       bool `json:"ok"`
			ExitCode int  `json:"exitCode"`
			Error    struct {
				Name    string `json:"name"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &badQuoting))
		require.False(t, badQuoting.OK)
		require.Equal(t, -1, badQuoting.ExitCode)
		require.Equal(t, "ValidationError", badQuoting.Error.Name)
		require.Contains(t, badQuoting.Error.Message, "unterminated")

		var badEnvLine struct {
			OK    bool `json:"ok"`
			Error struct {
				Name    string `json:"name"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &badEnvLine))
		require.False(t, badEnvLine.OK)
		require.Equal(t, "ValidationError", badEnvLine.Error.Name)
		require.Contains(t, badEnvLine.Error.Message, "KEY=value syntax")

		// A real ffmpeg failure (here, a nonexistent input file) reports
		// through the ordinary exitCode/stderr fields, with no "error"
		// object at all -- the process started and ran, it just failed.
		var badInputFile map[string]any
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &badInputFile))
		require.False(t, badInputFile["ok"].(bool))
		require.EqualValues(t, 254, badInputFile["exitCode"])
		require.NotContains(t, badInputFile, "error")
		require.Contains(t, badInputFile["stderr"], "No such file or directory")

		// with.timeoutSeconds sends SIGTERM; ffmpeg traps it and exits on
		// its own with its conventional signal-exit code (255) rather than
		// being killed outright, so exitCode is a real positive number even
		// though timedOut is true and ok is false.
		var timedOut struct {
			OK       bool `json:"ok"`
			ExitCode int  `json:"exitCode"`
			TimedOut bool `json:"timedOut"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &timedOut))
		require.False(t, timedOut.OK)
		require.True(t, timedOut.TimedOut)
		require.Equal(t, 255, timedOut.ExitCode)

		var truncated struct {
			OK        bool `json:"ok"`
			Truncated struct {
				Stdout bool `json:"stdout"`
				Stderr bool `json:"stderr"`
			} `json:"truncated"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 4)), &truncated))
		require.True(t, truncated.OK, "the command itself still succeeds; only its captured output is capped")
		require.True(t, truncated.Truncated.Stderr, "verbose ffmpeg logging should exceed the 1024-byte maxOutputBytes cap")
		require.False(t, truncated.Truncated.Stdout)
	})

	// Like node-script@v1, python-script@v1, and dbt@v1, a later step reads
	// a result field as ${<step id>.outputs.<name>} -- a bare-step-id
	// reference, not ${steps.<step id>.outputs.<name>} -- confirming this
	// is a property of remote action:-type steps generally, not specific to
	// one action.
	t.Run("a later step reads a result field as ${<step id>.outputs.<name>}, but not via steps.<id>.outputs.<name>", func(t *testing.T) {
		t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

		dagu := harness.NewRunner(t)
		result := dagu.Run("start", "downstream_reference.yaml")
		result.ExpectNonZeroExitCode()
		result.ExpectStderrContains("bad substitution")

		require.Equal(t, "bare exit code is 0\n", stepStdout(t, result.Stdout(), 1))
	})
}

// TestFfmpegValidation proves that dagu validate never resolves a remote
// action reference (which would require network access): an ffmpeg@v1 step
// passes validate regardless of its with: content, and even with.args --
// required by the action's own inputs schema -- is enforced only once the
// step actually runs and that schema is fetched. The one thing validate
// does check locally is the action reference's own syntax.
func TestFfmpegValidation(t *testing.T) {
	t.Parallel()

	t.Run("an ffmpeg@v1 step with no with.args passes validate", func(t *testing.T) {
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
		result.ExpectStderrContains(`unknown action "ffmpeg"`)
	})
}
