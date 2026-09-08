// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec048_duckdb_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestSourceGit(t *testing.T) {
	t.Parallel()

	port := startGitActionServer(t)
	env := append([]string{"HOST_SERVER_BASE=git://127.0.0.1:" + strconv.Itoa(port)}, gitEnv...)
	for _, tc := range []struct {
		fixture string
		errText string
	}{
		{"source_git_success.yaml", ""},
		{"source_git_bad_version.yaml", "checkout action source ref"},
		{"source_git_bad_repo.yaml", "clone action source"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.RunWithEnv(env, "start", tc.fixture)
			if tc.errText != "" {
				result.ExpectNonZeroExitCode()
				result.ExpectStderrContains(tc.errText)
				return
			}
			result.ExpectExitCode(0)

			var output struct {
				Echoed string `json:"echoed"`
			}
			require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &output))
			require.Equal(t, "hello-from-real-git-clone", output.Echoed)
		})
	}
}

func TestSourceGitValidation(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	dagu.Run("validate", "source_git_unreachable.yaml").ExpectExitCode(0)
}
