package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestMail(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "email-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test email attachment
	attachFile := filepath.Join(tmpDir, "email.txt")
	content := []byte("Test email")

	os.Setenv("MAIL_SUBJECT", "Test Subject")
	err = os.WriteFile(attachFile, content, 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	t.Cleanup(func() {
		os.Unsetenv("MAIL_SUBJECT")
		os.Remove(attachFile)
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
							"from":        "test@example.com",
							"to":          "recipient@example.com",
							"subject":     "Test Subject",
							"message":     "Test Message",
							"attachments": attachFile,
						},
					},
				},
			},
			{
				name: "ValidConfigWithEnv",
				step: digraph.Step{
					ExecutorConfig: digraph.ExecutorConfig{
						Config: map[string]any{
							"from":        "test@example.com",
							"to":          "recipient@example.com",
							"subject":     "${MAIL_SUBJECT}",
							"message":     "Test Message",
							"attachments": attachFile,
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
				}, nil, "", "", nil)

				exec, err := newMail(ctx, tt.step)

				assert.NoError(t, err)
				assert.NotNil(t, exec)

				mailExec, ok := exec.(*mail)
				assert.True(t, ok)
				assert.Equal(t, "test@example.com", mailExec.cfg.From)
				assert.Equal(t, "recipient@example.com", mailExec.cfg.To)
				assert.Equal(t, "Test Subject", mailExec.cfg.Subject)
				assert.Equal(t, "Test Message", mailExec.cfg.Message)
				assert.Equal(t, attachFile, mailExec.cfg.Attachments[0])
			})
		}
	})
}
