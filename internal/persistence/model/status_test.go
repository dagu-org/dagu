package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"

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
	startedAt, finishedAt := time.Now(), time.Now().Add(time.Second*1)
	dag := &digraph.DAG{
		HandlerOn: digraph.HandlerOn{},
		Steps: []digraph.Step{
			{
				Name: "1", Description: "",
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: digraph.ContinueOn{},
				RetryPolicy: digraph.RetryPolicy{}, MailOnError: false,
				RepeatPolicy: digraph.RepeatPolicy{}, Preconditions: []digraph.Condition{},
			},
		},
		MailOn:    &digraph.MailOn{},
		ErrorMail: &digraph.MailConfig{},
		InfoMail:  &digraph.MailConfig{},
		SMTP:      &digraph.SMTPConfig{},
	}
	requestID := "request-id-testI"
	statusToPersist := NewStatusFactory(dag).Create(
		requestID, scheduler.StatusSuccess, 0, startedAt, WithFinishedAt(finishedAt),
	)

	rawJSON, err := json.Marshal(statusToPersist)
	require.NoError(t, err)

	statusObject, err := StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, statusToPersist.Name, statusObject.Name)
	require.Equal(t, 1, len(statusObject.Nodes))
	require.Equal(t, dag.Steps[0].Name, statusObject.Nodes[0].Step.Name)
}

func TestCorrectRunningStatus(t *testing.T) {
	dag := &digraph.DAG{Name: "test"}
	requestID := "request-id-testII"
	status := NewStatusFactory(dag).Create(requestID, scheduler.StatusRunning, 0, time.Now())
	status.CorrectRunningStatus()
	require.Equal(t, scheduler.StatusError, status.Status)
}

func TestJsonMarshal(t *testing.T) {
	step := digraph.Step{
		OutputVariables: &digraph.SyncMap{},
	}
	step.OutputVariables.Store("A", "B")
	rawJSON, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log(string(rawJSON))
}
