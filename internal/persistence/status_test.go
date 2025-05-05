package persistence_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/google/uuid"

	"github.com/stretchr/testify/require"
)

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
	requestID := uuid.Must(uuid.NewV7()).String()
	statusToPersist := persistence.NewStatusFactory(dag).Create(
		requestID, scheduler.StatusSuccess, 0, startedAt, persistence.WithFinishedAt(finishedAt),
	)

	rawJSON, err := json.Marshal(statusToPersist)
	require.NoError(t, err)

	statusObject, err := persistence.StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, statusToPersist.Name, statusObject.Name)
	require.Equal(t, 1, len(statusObject.Nodes))
	require.Equal(t, dag.Steps[0].Name, statusObject.Nodes[0].Step.Name)
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
