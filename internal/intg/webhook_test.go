package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

// TestWebhookPayloadEnv tests that WEBHOOK_PAYLOAD environment variable
// is properly passed to DAG steps when triggered via webhook.
// Note: The actual webhook API integration tests require builtin auth mode
// with webhook store and are tested separately.
func TestWebhookPayloadEnv(t *testing.T) {
	t.Parallel()

	t.Run("AsEnvVar", func(t *testing.T) {
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

	t.Run("WithJsonContent", func(t *testing.T) {
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
