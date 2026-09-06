// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec037_docker_run_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// Configuration checks run without a container daemon or image downloads.
func TestDockerConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file      string
		command   string
		wantError string
	}{
		{"create_basic.yaml", "validate", ""},
		{"missing_target.yaml", "validate", "image"},
		{"missing_target_bare.yaml", "start", "docker step configuration is required"},
		{"exec_without_container_name.yaml", "validate", "container_name"},
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
