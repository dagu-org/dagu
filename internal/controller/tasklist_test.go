// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTaskListSummaryIncludesStableTaskNumbers(t *testing.T) {
	tasks := []Task{
		{
			Description: "Investigate the failing test",
			State:       TaskStateDone,
		},
		{
			Description: "Prepare fix\nRun targeted tests",
			State:       TaskStateOpen,
		},
	}

	got := buildTaskListSummary(tasks)

	require.Equal(t, "1. [x] Investigate the failing test\n2. [ ] Prepare fix\n   Run targeted tests", got)
}
