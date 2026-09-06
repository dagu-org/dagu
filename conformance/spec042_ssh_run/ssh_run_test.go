// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec042_ssh_run_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

func TestSSHConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file      string
		wantError string
	}{
		{"basic.yaml", ""},
		{"missing_command.yaml", "with.command is required"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.file)
			if tc.wantError == "" {
				result.ExpectExitCode(0)
			} else {
				result.ExpectNonZeroExitCode()
				result.ExpectStderrContains(tc.wantError)
			}
		})
	}
}
