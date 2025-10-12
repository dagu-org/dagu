package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/test"
)

func TestJQExecutor(t *testing.T) {
	t.Parallel()

	t.Run("MultipleOutputsWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-array-raw
    executor: 
      type: jq
      config:
        raw: true
    script: |
      { "data": [1, 2, 3] }
    command: '.data[]'
    output: RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("MultipleOutputsWithRawFalse", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-array-json
    executor: 
      type: jq
      config:
        raw: false
    script: |
      { "data": [1, 2, 3] }
    command: '.data[]'
    output: RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("StringOutputWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-strings-raw
    executor: 
      type: jq
      config:
        raw: true
    script: |
      { "messages": ["hello", "world"] }
    command: '.messages[]'
    output: RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "hello\nworld",
		})
	})

	t.Run("StringOutputWithRawFalse", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-strings-json
    executor: 
      type: jq
      config:
        raw: false
    script: |
      { "messages": ["hello", "world"] }
    command: '.messages[]'
    output: RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "\"hello\"\n\"world\"",
		})
	})

	t.Run("TSVOutputWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-tsv
    executor: 
      type: jq
      config:
        raw: true
    script: |
      { "data": [1, 2, 3] }
    command: '.data[] | [., 100 * .] | @tsv'
    output: RESULT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\t100\n2\t200\n3\t300",
		})
	})
}
