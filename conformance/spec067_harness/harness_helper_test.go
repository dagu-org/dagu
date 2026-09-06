// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec067_harness_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// requireLines asserts that content, split on newlines, equals exactly the
// given lines (with a trailing empty line implied by the final newline, as
// every fake_ok.sh invocation produces).
func requireLines(t *testing.T, content string, lines ...string) {
	t.Helper()
	want := strings.Join(lines, "\n") + "\n"
	require.Equal(t, want, content)
}

// requireContainsAll asserts that content contains every given substring.
func requireContainsAll(t *testing.T, content string, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		require.Contains(t, content, s)
	}
}

// stdoutLogPattern and stderrLogPattern match a per-step captured-output log
// path dagu start prints in its tree render, e.g.
// "└─stdout: /path/to/step.<ts>.<run>.out". One match appears per step that
// wrote anything to that stream, in the order dagu's tree render lists them
// (which, for both the sequential and the independent-but-declared-in-order
// fixtures this package uses, matches declaration order).
var stdoutLogPattern = regexp.MustCompile(`stdout: (.+)`)
var stderrLogPattern = regexp.MustCompile(`stderr: (.+)`)

func readLoggedPath(t *testing.T, pattern *regexp.Regexp, daguStartOutput string, n int) string {
	t.Helper()

	matches := pattern.FindAllStringSubmatch(daguStartOutput, -1)
	require.Greaterf(t, len(matches), n, "expected at least %d log paths in output:\n%s", n+1, daguStartOutput)
	path := strings.TrimSpace(matches[n][1])
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the harness's own trusted output.
	require.NoError(t, err)
	return string(data)
}

// stepStdout reads the exact bytes the (0-indexed) nth step with logged
// stdout wrote, by locating its captured-output log file from dagu start's
// own tree render and reading it directly, since the tree render re-wraps
// long lines with its own indentation, which would corrupt a strict content
// match.
func stepStdout(t *testing.T, daguStartOutput string, n int) string {
	t.Helper()
	return readLoggedPath(t, stdoutLogPattern, daguStartOutput, n)
}

// stepStderr is stepStdout's counterpart for a step's captured stderr.
func stepStderr(t *testing.T, daguStartOutput string, n int) string {
	t.Helper()
	return readLoggedPath(t, stderrLogPattern, daguStartOutput, n)
}

// fakeOkScript is a fake harness CLI: it prints one "ARG:<value>" line per
// argv element it received (so an argument containing spaces, such as a
// prompt, is unambiguous), then one "STDIN:<content>" line with whatever it
// read from stdin (empty when nothing was piped), and exits 0.
const fakeOkScript = `#!/bin/sh
for a in "$@"; do printf 'ARG:%s\n' "$a"; done
printf 'STDIN:%s\n' "$(cat)"
exit 0
`

// fakeFailScript is a fake harness CLI that always fails: it writes a fixed
// message to stderr and exits 7 (an arbitrary non-zero code distinct from
// the wrapper's own forced exit codes elsewhere in this session's specs).
const fakeFailScript = `#!/bin/sh
echo "fake_fail: simulated failure" >&2
exit 7
`

// writeFakeHarnessScripts writes the two fake CLI scripts this package's
// live fixtures reference (as ./scripts/fake_ok.sh and
// ./scripts/fake_fail.sh, relative to the DAG's own working_dir: field) and
// returns the host environment entry the fixtures resolve their DAG-level
// working_dir: from. A harnesses.<name>.binary value does not go through
// Dagu's own value resolution (unlike almost every other field), so the
// fixtures cannot embed the isolated project's own runtime path directly in
// binary: -- only working_dir: does, which is why binary: is always a
// static relative path here.
func writeFakeHarnessScripts(dagu *harness.Runner) []string {
	dagu.WriteExecutable("scripts/fake_ok.sh", fakeOkScript)
	dagu.WriteExecutable("scripts/fake_fail.sh", fakeFailScript)
	return []string{"HOST_PROJECT_DIR=" + dagu.ProjectPath("")}
}
