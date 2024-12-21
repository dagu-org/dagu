// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
)

func TestStartCommand(t *testing.T) {
	th := testSetup(t)
	tests := []cmdTest{
		{
			name:        "StartDAG",
			args:        []string{"start", th.DAGFile("success.yaml").Path},
			expectedOut: []string{"Step execution started"},
		},
		{
			name:        "StartDAGWithDefaultParams",
			args:        []string{"start", th.DAGFile("params.yaml").Path},
			expectedOut: []string{`params="[p1 p2]"`},
		},
		{
			name:        "StartDAGWithParams",
			args:        []string{"start", `--params="p3 p4"`, th.DAGFile("params.yaml").Path},
			expectedOut: []string{`params="[p3 p4]"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th.RunCommand(t, startCmd(), tc)
		})
	}
}
