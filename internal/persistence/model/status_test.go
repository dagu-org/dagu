package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"

	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestPid(t *testing.T) {
	if PidNotRunning.IsRunning() {
		t.Error()
	}
	var pid Pid = Pid(-1)
	require.Equal(t, "", pid.String())

	pid = Pid(12345)
	require.Equal(t, "12345", pid.String())
}

func TestStatusSerialization(t *testing.T) {
	start, end := time.Now(), time.Now().Add(time.Second*1)
	d := &dag.DAG{
		Location:    "",
		Name:        "",
		Description: "",
		Env:         []string{},
		LogDir:      "",
		HandlerOn:   dag.HandlerOn{},
		Steps: []*dag.Step{
			{
				Name: "1", Description: "", Variables: []string{},
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: dag.ContinueOn{},
				RetryPolicy: &dag.RetryPolicy{}, MailOnError: false,
				RepeatPolicy: dag.RepeatPolicy{}, Preconditions: []*dag.Condition{},
			},
		},
		MailOn:            &dag.MailOn{},
		ErrorMail:         &dag.MailConfig{},
		InfoMail:          &dag.MailConfig{},
		Smtp:              &dag.SmtpConfig{},
		Delay:             0,
		HistRetentionDays: 0,
		Preconditions:     []*dag.Condition{},
		MaxActiveRuns:     0,
		Params:            []string{},
		DefaultParams:     "",
	}
	st := NewStatus(d, nil, scheduler.SchedulerStatus_Success, 10000, &start, &end)

	js, err := st.ToJson()
	require.NoError(t, err)

	st_, err := StatusFromJson(string(js))
	require.NoError(t, err)

	require.Equal(t, st.Name, st_.Name)
	require.Equal(t, 1, len(st_.Nodes))
	require.Equal(t, d.Steps[0].Name, st_.Nodes[0].Name)
}

func TestCorrectRunningStatus(t *testing.T) {
	d := &dag.DAG{Name: "test"}
	status := NewStatus(d, nil, scheduler.SchedulerStatus_Running,
		10000, nil, nil)
	status.CorrectRunningStatus()
	require.Equal(t, scheduler.SchedulerStatus_Error, status.Status)
}

func TestJsonMarshal(t *testing.T) {
	step := dag.Step{
		OutputVariables: &utils.SyncMap{},
	}
	step.OutputVariables.Store("A", "B")
	js, err := json.Marshal(step)
	if err != nil {
		t.Fatalf(err.Error())
	}
	t.Logf(string(js))
}
