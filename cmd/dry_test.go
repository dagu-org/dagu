// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		tests := []cmdTest{
			{
				args:        []string{"dry", testDAGFile("success.yaml")},
				expectedOut: []string{"Starting DRY-RUN"},
			},
		}

		for _, tc := range tests {
			testRunCommand(t, dryCmd(), tc)
		}
	})
}
