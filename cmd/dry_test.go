// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := testSetup(t)
		tests := []cmdTest{
			{
				name:        "DryRunDAG",
				args:        []string{"dry", th.DAGFile("success.yaml").Path},
				expectedOut: []string{"Dry-run finished"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				th.RunCommand(t, dryCmd(), tc)
			})
		}
	})
}
