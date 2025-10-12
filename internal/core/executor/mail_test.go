package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestMail(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "email-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a test email attachment
	attachFile := filepath.Join(tmpDir, "email.txt")
	content := []byte("Test email")

	err = os.WriteFile(attachFile, content, 0600)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Remove(attachFile)
	})

	t.Run("NewMail", func(t *testing.T) {
		tests := []struct {
			name string
			step core.Step
		}{
			{
				name: "ValidConfig",
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
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
				step: core.Step{
					ExecutorConfig: core.ExecutorConfig{
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
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				ctx = core.SetupDAGContext(ctx, &core.DAG{
					SMTP: &core.SMTPConfig{},
				}, nil, core.DAGRunRef{}, "", "", nil, nil)

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

	t.Run("MultipleRecipients", func(t *testing.T) {
		tests := []struct {
			name      string
			toField   any
			expected  []string
			expectErr bool
		}{
			{
				name:     "SingleRecipientString",
				toField:  "single@example.com",
				expected: []string{"single@example.com"},
			},
			{
				name:     "MultipleRecipientsArray",
				toField:  []string{"user1@example.com", "user2@example.com", "user3@example.com"},
				expected: []string{"user1@example.com", "user2@example.com", "user3@example.com"},
			},
			{
				name:     "MultipleRecipientsAnyArray",
				toField:  []any{"user1@example.com", "user2@example.com"},
				expected: []string{"user1@example.com", "user2@example.com"},
			},
			{
				name:      "EmptyString",
				toField:   "",
				expected:  nil,
				expectErr: true, // Should error because no valid recipients
			},
			{
				name:      "EmptyArray",
				toField:   []string{},
				expected:  nil,
				expectErr: true, // Should error because no valid recipients
			},
			{
				name:     "ArrayWithEmptyStrings",
				toField:  []any{"user1@example.com", "", "user2@example.com"},
				expected: []string{"user1@example.com", "user2@example.com"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				step := core.Step{
					ExecutorConfig: core.ExecutorConfig{
						Config: map[string]any{
							"from":    "test@example.com",
							"to":      tt.toField,
							"subject": "Test Subject",
							"message": "Test Message",
						},
					},
				}

				ctx := context.Background()
				ctx = core.SetupDAGContext(ctx, &core.DAG{
					SMTP: &core.SMTPConfig{},
				}, nil, core.DAGRunRef{}, "", "", nil, nil)

				exec, err := newMail(ctx, step)
				assert.NoError(t, err)
				assert.NotNil(t, exec)

				mailExec, ok := exec.(*mail)
				assert.True(t, ok)

				// Test Run method to validate the to field handling
				if tt.expectErr {
					err := mailExec.Run(ctx)
					assert.Error(t, err)
				} else {
					// We can't actually run the mail sending without mocking,
					// but we can verify the config is set correctly
					assert.Equal(t, tt.toField, mailExec.cfg.To)
				}
			})
		}
	})
}
