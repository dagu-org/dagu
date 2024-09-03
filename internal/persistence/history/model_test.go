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
	status := NewStatus(dAG, nil, scheduler.StatusSuccess, 10000, &start, &end)

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
	status := NewStatus(dAG, nil, scheduler.StatusRunning,
		10000, nil, nil)
	status.CorrectRunningStatus()
	require.Equal(t, scheduler.StatusError, status.Status)
}
