package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/dag/scheduler"

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
	dg := &dag.DAG{
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
	status := NewStatus(dg, nil, scheduler.StatusSuccess, 10000, &start, &end)

	rawJSON, err := status.ToJSON()
	require.NoError(t, err)

	unmarshalled, err := StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, status.Name, unmarshalled.Name)
	require.Equal(t, 1, len(unmarshalled.Nodes))
	require.Equal(t, dg.Steps[0].Name, unmarshalled.Nodes[0].Name)
}

func TestCorrectRunningStatus(t *testing.T) {
	dg := &dag.DAG{Name: "test"}
	status := NewStatus(dg, nil, scheduler.StatusRunning,
		10000, nil, nil)
	status.CorrectRunningStatus()
	require.Equal(t, scheduler.StatusError, status.Status)
}

func TestJsonMarshal(t *testing.T) {
	step := dag.Step{
		OutputVariables: &dag.SyncMap{},
	}
	step.OutputVariables.Store("A", "B")
	rawJSON, err := json.Marshal(step)
	if err != nil {
		t.Fatalf(err.Error())
	}
	t.Logf(string(rawJSON))
}
