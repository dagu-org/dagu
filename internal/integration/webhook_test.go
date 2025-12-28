package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWebhookConfig(t *testing.T) {
	t.Run("ParseWebhookConfig", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Create a DAG with webhook configuration
		dag := th.DAG(t, `
name: webhook-test
webhook:
  enabled: true
  token: my-secret-token-1234

steps:
  - name: test
    command: echo "test"
`)

		// Verify the webhook config was parsed correctly
		require.NotNil(t, dag.Webhook)
		require.True(t, dag.Webhook.Enabled)
		require.Equal(t, "my-secret-token-1234", dag.Webhook.Token)
	})

	t.Run("ParseWebhookConfigWithEnvVar", func(t *testing.T) {
		// Test that environment variables in the token are expanded
		// Note: t.Setenv requires non-parallel test
		t.Setenv("MY_WEBHOOK_TOKEN", "expanded-secret-12345")

		th := test.Setup(t)

		// Create a DAG with webhook configuration using env var
		dag := th.DAG(t, `
name: webhook-env-test
webhook:
  enabled: true
  token: ${MY_WEBHOOK_TOKEN}

steps:
  - name: test
    command: echo "test"
`)

		// Verify the webhook config was parsed correctly with env expansion
		require.NotNil(t, dag.Webhook)
		require.True(t, dag.Webhook.Enabled)
		require.Equal(t, "expanded-secret-12345", dag.Webhook.Token)
	})

	t.Run("ParseWebhookDisabled", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Create a DAG with webhook disabled
		dag := th.DAG(t, `
name: webhook-disabled-test
webhook:
  enabled: false
  token: some-token-12345678

steps:
  - name: test
    command: echo "test"
`)

		// Verify the webhook is disabled
		require.NotNil(t, dag.Webhook)
		require.False(t, dag.Webhook.Enabled)
	})

	t.Run("ParseWebhookNotConfigured", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Create a DAG without webhook configuration
		dag := th.DAG(t, `
name: no-webhook-test

steps:
  - name: test
    command: echo "test"
`)

		// Verify the webhook is nil
		require.Nil(t, dag.Webhook)
	})
}

func TestWebhookPayloadEnv(t *testing.T) {
	t.Parallel()

	t.Run("WebhookPayloadAsEnvVar", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Create a DAG that uses WEBHOOK_PAYLOAD
		// Note: The shell echo command doesn't preserve JSON quotes,
		// so we just check that the env var content is passed correctly
		dag := th.DAG(t, `
name: webhook-payload-test

params:
  - WEBHOOK_PAYLOAD: 'test-payload-value'

steps:
  - name: process-webhook
    command: echo "$WEBHOOK_PAYLOAD"
    output: PAYLOAD_OUTPUT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"PAYLOAD_OUTPUT": "test-payload-value",
		})
	})

	t.Run("WebhookPayloadWithJsonContent", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Create a DAG that processes JSON payload using shell command
		// This simulates how a real webhook would work
		dag := th.DAG(t, `
name: webhook-json-test

params:
  - WEBHOOK_PAYLOAD: '{"event":"push","repository":"dagu"}'

steps:
  - name: check-payload-contains-event
    command: |
      if echo "$WEBHOOK_PAYLOAD" | grep -q "event"; then
        echo "found"
      else
        echo "not-found"
      fi
    output: CHECK_RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"CHECK_RESULT": "found",
		})
	})
}

func TestWebhookSpecValidation(t *testing.T) {
	t.Parallel()

	t.Run("ValidWebhookSpec", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Valid webhook configuration should not produce errors
		content := []byte(`
name: valid-webhook
webhook:
  enabled: true
  token: test-token-12345678

steps:
  - name: test
    command: echo "test"
`)

		dag, err := spec.LoadYAML(th.Context, content, spec.WithName("valid-webhook"))
		require.NoError(t, err)
		require.NotNil(t, dag)
		require.NotNil(t, dag.Webhook)
		require.True(t, dag.Webhook.Enabled)
		require.Equal(t, "test-token-12345678", dag.Webhook.Token)
	})

	t.Run("WebhookEnabledWithMissingToken", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Webhook enabled but token missing should fail
		content := []byte(`
name: webhook-missing-token
webhook:
  enabled: true

steps:
  - name: test
    command: echo "test"
`)

		_, err := spec.LoadYAML(th.Context, content, spec.WithName("webhook-missing-token"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "token is required when webhook is enabled")
	})

	t.Run("WebhookTokenTooShort", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Token with less than 16 characters should fail
		content := []byte(`
name: webhook-short-token
webhook:
  enabled: true
  token: short-token

steps:
  - name: test
    command: echo "test"
`)

		_, err := spec.LoadYAML(th.Context, content, spec.WithName("webhook-short-token"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "token must be at least 16 characters")
	})

	t.Run("WebhookTokenExactlyMinLength", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Token with exactly 16 characters should succeed
		content := []byte(`
name: webhook-min-token
webhook:
  enabled: true
  token: "1234567890abcdef"

steps:
  - name: test
    command: echo "test"
`)

		dag, err := spec.LoadYAML(th.Context, content, spec.WithName("webhook-min-token"))
		require.NoError(t, err)
		require.NotNil(t, dag)
		require.NotNil(t, dag.Webhook)
		require.True(t, dag.Webhook.Enabled)
		require.Equal(t, "1234567890abcdef", dag.Webhook.Token)
	})

	t.Run("WebhookDisabledWithShortToken", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// When webhook is disabled, short token should be allowed (no validation)
		content := []byte(`
name: webhook-disabled-short-token
webhook:
  enabled: false
  token: short

steps:
  - name: test
    command: echo "test"
`)

		dag, err := spec.LoadYAML(th.Context, content, spec.WithName("webhook-disabled-short-token"))
		require.NoError(t, err)
		require.NotNil(t, dag)
		require.NotNil(t, dag.Webhook)
		require.False(t, dag.Webhook.Enabled)
	})
}
