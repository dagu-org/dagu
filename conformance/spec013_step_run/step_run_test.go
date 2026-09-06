// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec013_step_run_test

import (
	"runtime"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// TestArrayFormMalformedEntriesAreRejected proves the "Field Shape" rules for
// array-form `run`: a mapping item with more than one key, and a single-key
// mapping item whose value is itself a nested mapping or a nested array,
// must all fail validation rather than being silently coerced.
func TestArrayFormMalformedEntriesAreRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
	}{
		{name: "multi-key mapping item", file: "array_form_multi_key_mapping_invalid.yaml"},
		{name: "nested mapping value", file: "array_form_nested_mapping_value_invalid.yaml"},
		{name: "nested array item", file: "array_form_nested_array_item_invalid.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.file)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains("command")
		})
	}
}

// TestArrayFormSingleKeyScalarMappingIsAccepted proves the companion positive
// rule: a single-key mapping item whose value is a primitive scalar is valid
// and converts to a "key: value" command string, run in order alongside
// plain string entries.
func TestArrayFormSingleKeyScalarMappingIsAccepted(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "array_form_single_key_scalar_valid.yaml")
	result.ExpectExitCode(0)
	dagu.ExpectFileContent("out.txt", "first\nhello: world\n")
}

// TestShellSelectionPrecedence proves the full selection order from "Shell
// Selection": step with.shell beats root shell, root shell beats
// DAGU_DEFAULT_SHELL, and DAGU_DEFAULT_SHELL beats the platform discovery
// fallback (here poisoned via $SHELL). Each fixture poisons every lower-
// precedence source with a nonexistent path, so the run only succeeds if the
// higher-precedence source was actually the one selected.
func TestShellSelectionPrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixtures assume a POSIX sh is available")
	}
	t.Parallel()

	t.Run("step with.shell overrides root shell and DAGU_DEFAULT_SHELL", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.RunWithEnv(
			[]string{"DAGU_DEFAULT_SHELL=/nonexistent/poison-default-shell"},
			"start", "shell_step_overrides_root_default.yaml",
		)
		result.ExpectExitCode(0)
		dagu.ExpectFileContent("result.out", "ok\n")
	})

	t.Run("root shell overrides DAGU_DEFAULT_SHELL", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.RunWithEnv(
			[]string{"DAGU_DEFAULT_SHELL=/nonexistent/poison-default-shell"},
			"start", "shell_selection_root_overrides_default.yaml",
		)
		result.ExpectExitCode(0)
		dagu.ExpectFileContent("result.out", "ok\n")
	})

	t.Run("DAGU_DEFAULT_SHELL overrides the platform fallback", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.RunWithEnv(
			[]string{"SHELL=/nonexistent/poison-shell-env", "DAGU_DEFAULT_SHELL=sh"},
			"start", "shell_default_overrides_platform.yaml",
		)
		result.ExpectExitCode(0)
		dagu.ExpectFileContent("result.out", "ok\n")
	})
}
