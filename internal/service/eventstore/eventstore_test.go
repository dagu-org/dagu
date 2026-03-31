// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestPersistedDAGRunEventTypeForStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status core.Status
		want   EventType
		ok     bool
	}{
		{name: "NotStarted", status: core.NotStarted, ok: false},
		{name: "Queued", status: core.Queued, ok: false},
		{name: "Running", status: core.Running, ok: false},
		{name: "Rejected", status: core.Rejected, ok: false},
		{name: "Waiting", status: core.Waiting, want: TypeDAGRunWaiting, ok: true},
		{name: "Succeeded", status: core.Succeeded, want: TypeDAGRunSucceeded, ok: true},
		{name: "PartiallySucceeded", status: core.PartiallySucceeded, want: TypeDAGRunSucceeded, ok: true},
		{name: "Failed", status: core.Failed, want: TypeDAGRunFailed, ok: true},
		{name: "Aborted", status: core.Aborted, want: TypeDAGRunAborted, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PersistedDAGRunEventTypeForStatus(tt.status)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
