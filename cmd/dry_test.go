// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.Setup(t)

		tests := []cmdTest{
			{
				args:        []string{"dry", testDAGFile("success.yaml")},
				expectedOut: []string{"Starting DRY-RUN"},
			},
		}

		for _, tc := range tests {
			testRunCommand(t, th.Context, dryCmd(), tc)
		}
	})
}
