package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestMailConfigEnvExpansion(t *testing.T) {
	t.Parallel()

	t.Run("SMTPConfigWithSecrets", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Create a secret file for SMTP password
		smtpPassword := "super-secret-smtp-password"
		secretFile := th.TempFile(t, "smtp-password.txt", []byte(smtpPassword))

		// Test that SMTP config with secrets is properly evaluated
		// The DAG should run successfully even though we're not actually sending mail
		dag := th.DAG(t, `
secrets:
  - name: SMTP_PASSWORD
    provider: file
    key: `+secretFile+`

env:
  - SMTP_HOST: smtp.example.com

smtp:
  host: ${SMTP_HOST}
  port: "587"
  username: user@example.com
  password: ${SMTP_PASSWORD}

steps:
  - name: test-step
    command: echo "SMTP config evaluated successfully"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertOutputs(t, map[string]any{
			"RESULT": "SMTP config evaluated successfully",
		})

		// Verify the SMTP config was evaluated (password should not contain ${...})
		require.NotNil(t, dag.DAG.SMTP)
		require.Equal(t, "smtp.example.com", dag.DAG.SMTP.Host)
		require.Equal(t, smtpPassword, dag.DAG.SMTP.Password)
	})

	t.Run("WaitMailConfigWithEnvVars", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		dag := th.DAG(t, `
env:
  - WAIT_PREFIX: "[Waiting]"
  - OPS_EMAIL: ops@example.com

waitMail:
  from: alerts@example.com
  to: ${OPS_EMAIL}
  prefix: ${WAIT_PREFIX}

steps:
  - name: test-step
    command: echo "WaitMail config evaluated successfully"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertOutputs(t, map[string]any{
			"RESULT": "WaitMail config evaluated successfully",
		})

		// Verify the waitMail config was evaluated
		require.NotNil(t, dag.DAG.WaitMail)
		require.Equal(t, "alerts@example.com", dag.DAG.WaitMail.From)
		require.Equal(t, []string{"ops@example.com"}, dag.DAG.WaitMail.To)
		require.Equal(t, "[Waiting]", dag.DAG.WaitMail.Prefix)
	})

	t.Run("MultipleRecipientsWithEnvVars", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		dag := th.DAG(t, `
env:
  - ADMIN1: admin1@example.com
  - ADMIN2: admin2@example.com

errorMail:
  from: alerts@example.com
  to:
    - ${ADMIN1}
    - ${ADMIN2}
  prefix: "[Alert]"

steps:
  - name: test-step
    command: echo "Multiple recipients evaluated successfully"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertOutputs(t, map[string]any{
			"RESULT": "Multiple recipients evaluated successfully",
		})

		// Verify multiple recipients were evaluated
		require.NotNil(t, dag.DAG.ErrorMail)
		require.Equal(t, []string{"admin1@example.com", "admin2@example.com"}, dag.DAG.ErrorMail.To)
	})

	t.Run("AllMailConfigsWithMixedSources", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Create secrets
		smtpPassword := "smtp-pass-123"
		adminEmail := "secret-admin@corp.com"
		smtpPassFile := th.TempFile(t, "smtp-pass.txt", []byte(smtpPassword))
		adminEmailFile := th.TempFile(t, "admin-email.txt", []byte(adminEmail))

		dag := th.DAG(t, `
env:
  - SMTP_HOST: mail.corp.com
  - SMTP_USER: mailbot@corp.com
  - OPS_EMAIL: ops@corp.com

secrets:
  - name: SMTP_PASS
    provider: file
    key: `+smtpPassFile+`
  - name: ADMIN_EMAIL
    provider: file
    key: `+adminEmailFile+`

smtp:
  host: ${SMTP_HOST}
  port: "465"
  username: ${SMTP_USER}
  password: ${SMTP_PASS}

errorMail:
  from: ${SMTP_USER}
  to:
    - ${ADMIN_EMAIL}
    - ${OPS_EMAIL}
  prefix: "[ERROR]"

infoMail:
  from: ${SMTP_USER}
  to: ${OPS_EMAIL}
  prefix: "[INFO]"

steps:
  - name: test-step
    command: echo "All mail configs evaluated successfully"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertOutputs(t, map[string]any{
			"RESULT": "All mail configs evaluated successfully",
		})

		// Verify SMTP config
		require.NotNil(t, dag.DAG.SMTP)
		require.Equal(t, "mail.corp.com", dag.DAG.SMTP.Host)
		require.Equal(t, "mailbot@corp.com", dag.DAG.SMTP.Username)
		require.Equal(t, smtpPassword, dag.DAG.SMTP.Password)

		// Verify errorMail config
		require.NotNil(t, dag.DAG.ErrorMail)
		require.Equal(t, "mailbot@corp.com", dag.DAG.ErrorMail.From)
		require.Equal(t, []string{adminEmail, "ops@corp.com"}, dag.DAG.ErrorMail.To)

		// Verify infoMail config
		require.NotNil(t, dag.DAG.InfoMail)
		require.Equal(t, "mailbot@corp.com", dag.DAG.InfoMail.From)
		require.Equal(t, []string{"ops@corp.com"}, dag.DAG.InfoMail.To)
	})
}
