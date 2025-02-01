package executor

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestMail(t *testing.T) {
	t.Parallel()

	os.Setenv("MAIL_SUBJECT", "Test Subject")
	t.Cleanup(func() {
		os.Unsetenv("MAIL_SUBJECT")
	})

	t.Run("NewMail", func(t *testing.T) {
		tests := []struct {
			name string
			step digraph.Step
		}{
			{
				name: "ValidConfig",
				step: digraph.Step{
					ExecutorConfig: digraph.ExecutorConfig{
						Config: map[string]any{
							"from":    "test@example.com",
							"to":      "recipient@example.com",
							"subject": "Test Subject",
							"message": "Test Message",
						},
					},
				},
			},
			{
				name: "ValidConfigWithEnv",
				step: digraph.Step{
					ExecutorConfig: digraph.ExecutorConfig{
						Config: map[string]any{
							"from":    "test@example.com",
							"to":      "recipient@example.com",
							"subject": "${MAIL_SUBJECT}",
							"message": "Test Message",
						},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				ctx = digraph.NewContext(ctx, &digraph.DAG{
					SMTP: &digraph.SMTPConfig{},
				}, nil, "", "")

				exec, err := newMail(ctx, tt.step)

				assert.NoError(t, err)
				assert.NotNil(t, exec)

				mailExec, ok := exec.(*mail)
				assert.True(t, ok)
				assert.Equal(t, "test@example.com", mailExec.cfg.From)
				assert.Equal(t, "recipient@example.com", mailExec.cfg.To)
				assert.Equal(t, "Test Subject", mailExec.cfg.Subject)
				assert.Equal(t, "Test Message", mailExec.cfg.Message)
			})
		}
	})
}
