// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec048_duckdb_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// A local bundle exercises the action boundary without downloads.
func TestLocalAction(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "local_action_success.yaml")
	result.ExpectExitCode(0)
	data, err := os.ReadFile(dagu.ProjectPath("result.json"))
	require.NoError(t, err)
	var output map[string]string
	require.NoError(t, json.Unmarshal(data, &output))
	require.Equal(t, "hello-from-local-action", output["echoed"])
}

func TestActionBoundary(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ file, command, wantError string }{
		{"duckdb_live.yaml", "validate", ""},
		{"source_missing_version.yaml", "validate", "source action references"},
		{"local_action_missing_input.yaml", "start", "action input does not match inputs schema"},
		{"local_action_bad_output.yaml", "start", "action output does not match outputs schema"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run(tc.command, tc.file)
			if tc.wantError == "" {
				result.ExpectExitCode(0)
			} else {
				result.ExpectNonZeroExitCode()
				result.ExpectStderrContains(tc.wantError)
			}
		})
	}
}
