// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestPreservedQueueTriggerType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *DAGRunStatus
		want   core.TriggerType
	}{
		{
			name: "QueuedCatchup",
			status: &DAGRunStatus{
				Status:      core.Queued,
				TriggerType: core.TriggerTypeCatchUp,
			},
			want: core.TriggerTypeCatchUp,
		},
		{
			name: "QueuedController",
			status: &DAGRunStatus{
				Status:      core.Queued,
				TriggerType: core.TriggerTypeController,
			},
			want: core.TriggerTypeController,
		},
		{
			name: "QueuedManual",
			status: &DAGRunStatus{
				Status:      core.Queued,
				TriggerType: core.TriggerTypeManual,
			},
			want: core.TriggerTypeUnknown,
		},
		{
			name: "FinishedController",
			status: &DAGRunStatus{
				Status:      core.Succeeded,
				TriggerType: core.TriggerTypeController,
			},
			want: core.TriggerTypeUnknown,
		},
		{
			name: "Nil",
			want: core.TriggerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PreservedQueueTriggerType(tt.status))
		})
	}
}
