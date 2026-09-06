// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec055_git_test covers local git checkout wiring through the CLI.
package spec055_git_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestGitCheckout(t *testing.T) {
	t.Parallel()
	origin := setupOriginRepo(t)
	dagu := harness.NewRunner(t)
	dagu.RunWithEnv([]string{"GIT_ORIGIN=" + origin.Path}, "start", "checkout.yaml").ExpectExitCode(0)
	dagu.ExpectFileContent("work/file.txt", "v1")

	for _, tc := range []struct {
		file    string
		commit  string
		cloned  bool
		changed bool
	}{
		{"clone.json", origin.HeadCommit, true, true},
		{"repeat.json", origin.HeadCommit, false, false},
		{"tag.json", origin.FirstCommit, false, true},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(dagu.ProjectPath(tc.file))
			require.NoError(t, err)
			var out struct {
				Operation string `json:"operation"`
				Commit    string `json:"commit"`
				Cloned    bool   `json:"cloned"`
				Changed   bool   `json:"changed"`
			}
			require.NoError(t, json.Unmarshal(data, &out))
			require.Equal(t, "checkout", out.Operation)
			require.Equal(t, tc.commit, out.Commit)
			require.Equal(t, tc.cloned, out.Cloned)
			require.Equal(t, tc.changed, out.Changed)
		})
	}
}

func TestGitCheckoutValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ file, message string }{
		{"missing_repository.yaml", "repository is required"},
		{"missing_path.yaml", "path is required"},
		{"negative_depth.yaml", "minimum"},
		{"ssh_key_and_token.yaml", "ssh_key_path cannot be combined with token or password"},
		{"token_and_username.yaml", "token cannot be combined with username/password"},
		{"unsupported_field.yaml", "invalid keys: branch"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.file)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.message)
		})
	}
}
