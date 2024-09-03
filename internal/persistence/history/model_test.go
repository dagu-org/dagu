// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package history

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/stretchr/testify/require"
)

func TestPID(t *testing.T) {
	if pidNotRunning.IsRunning() {
		t.Error()
	}
	var pid = PID(-1)
	require.Equal(t, "", pid.String())

	pid = PID(12345)
	require.Equal(t, "12345", pid.String())
}

func TestStatusSerialization(t *testing.T) {
	start, end := time.Now(), time.Now().Add(time.Second*1)
	dAG := &dag.DAG{
		HandlerOn: dag.HandlerOn{},
		Steps: []dag.Step{
			{
				Name: "1", Description: "", Variables: []string{},
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: dag.ContinueOn{},
				RetryPolicy: &dag.RetryPolicy{}, MailOnError: false,
				RepeatPolicy: dag.RepeatPolicy{}, Preconditions: []dag.Condition{},
			},
		},
		MailOn:    &dag.MailOn{},
		ErrorMail: &dag.MailConfig{},
		InfoMail:  &dag.MailConfig{},
		SMTP:      &dag.SMTPConfig{},
	}
	status := NewStatus(NewStatusArgs{
		DAG:        dAG,
		Status:     scheduler.StatusSuccess,
		PID:        10000,
		StartedAt:  start,
		FinishedAt: end,
	})

	rawJSON, err := status.ToJSON()
	require.NoError(t, err)

	unmarshalled, err := StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, status.Name, unmarshalled.Name)
	require.Equal(t, 1, len(unmarshalled.Nodes))
	require.Equal(t, dAG.Steps[0].Name, unmarshalled.Nodes[0].Step.Name)
}

func TestCorrectRunningStatus(t *testing.T) {
	dAG := &dag.DAG{Name: "test"}
	status := NewStatus(NewStatusArgs{
		DAG:    dAG,
		Status: scheduler.StatusRunning,
		PID:    10000,
	})
	status.CorrectRunningStatus()
	require.Equal(t, scheduler.StatusError, status.Status)
}
