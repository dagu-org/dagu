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
// each fixture script produces).
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
// message to stderr and exits 7 to exercise process failure.
const fakeFailScript = `#!/bin/sh
echo "fake_fail: simulated failure" >&2
exit 7
`

// writeFakeHarnessScripts supplies local scripts invoked through sh on every OS.
func writeFakeHarnessScripts(dagu *harness.Runner) []string {
	dagu.WriteExecutable("scripts/fake_ok.sh", fakeOkScript)
	dagu.WriteExecutable("scripts/fake_fail.sh", fakeFailScript)
	return []string{"HOST_PROJECT_DIR=" + dagu.ProjectPath("")}
}
