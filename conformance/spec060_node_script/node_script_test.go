// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec060_node_script_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// Validation checks the remote reference without provisioning its runtime.
func TestNodeScriptConfig(t *testing.T) {
	t.Parallel()

	t.Run("versioned action", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		dagu.Run("validate", "configured.yaml").ExpectExitCode(0)
	})

	t.Run("missing version", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.Run("validate", "no_version_suffix.yaml")
		result.ExpectNonZeroExitCode()
		result.ExpectStderrContains(`unknown action "node-script"`)
	})
}
