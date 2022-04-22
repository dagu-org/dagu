package models_test

import (
	"jobctl/internal/config"
	"jobctl/internal/models"
	"jobctl/internal/scheduler"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPid(t *testing.T) {
	if models.PidNotRunning.IsRunning() {
		t.Error()
	}
}

func TestStatusSerialization(t *testing.T) {
	start, end := time.Now(), time.Now().Add(time.Second*1)
	cfg := &config.Config{
		ConfigPath:  "",
		Name:        "",
		Description: "",
		Env:         []string{},
		LogDir:      "",
		HandlerOn:   config.HandlerOn{},
		Steps: []*config.Step{
			{
				Name: "1", Description: "", Variables: []string{},
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: config.ContinueOn{},
				RetryPolicy: &config.RetryPolicy{}, MailOnError: false,
				Repeat: false, RepeatInterval: 0, Preconditions: []*config.Condition{},
			},
		},
		MailOn:            config.MailOn{},
		ErrorMail:         &config.MailConfig{},
		InfoMail:          &config.MailConfig{},
		Smtp:              &config.SmtpConfig{},
		DelaySec:          0,
		HistRetentionDays: 0,
		Preconditions:     []*config.Condition{},
		MaxActiveRuns:     0,
		Params:            []string{},
		DefaultParams:     "",
	}
	st := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_Success, 10000, &start, &end)

	js, err := st.ToJson()
	require.NoError(t, err)

	st_, err := models.StatusFromJson(string(js))
	require.NoError(t, err)

	assert.Equal(t, st.Name, st_.Name)
	require.Equal(t, 1, len(st_.Nodes))
	assert.Equal(t, cfg.Steps[0].Name, st_.Nodes[0].Name)
}
