package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
)

func TestJQExecutor(t *testing.T) {
	t.Parallel()

	t.Run("MultipleOutputsWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-array-raw
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

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("MultipleOutputsWithRawFalse", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-array-json
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

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\n2\n3",
		})
	})

	t.Run("StringOutputWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-strings-raw
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

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "hello\nworld",
		})
	})

	t.Run("StringOutputWithRawFalse", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-strings-json
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

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "\"hello\"\n\"world\"",
		})
	})

	t.Run("TSVOutputWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-tsv
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

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "1\t100\n2\t200\n3\t300",
		})
	})

	t.Run("SingleStringWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-single-string-raw
    type: jq
    config:
      raw: true
    script: |
      {"foo": "bar"}
    command: .foo
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "bar",
		})
	})

	t.Run("SingleStringWithRawFalse", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-single-string-json
    type: jq
    config:
      raw: false
    script: |
      {"foo": "bar"}
    command: .foo
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": `"bar"`,
		})
	})

	t.Run("SingleNumberWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-single-number-raw
    type: jq
    config:
      raw: true
    script: |
      {"value": 42}
    command: .value
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "42",
		})
	})

	t.Run("SingleBooleanWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-single-boolean-raw
    type: jq
    config:
      raw: true
    script: |
      {"enabled": true, "disabled": false}
    command: .enabled
    output: ENABLED
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"ENABLED": "true",
		})
	})

	t.Run("NullValueWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-null-raw
    type: jq
    config:
      raw: true
    script: |
      {"value": null}
    command: .value
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// Null values output empty string, but the output variable still exists
		// So we check that it contains an empty value
		dag.AssertOutputs(t, map[string]any{
			"RESULT": test.Contains("RESULT="),
		})
	})

	t.Run("ObjectWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-object-raw
    type: jq
    config:
      raw: true
    script: |
      {"user": {"name": "John", "age": 30}}
    command: .user
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// In raw mode, object is output as compact JSON (key order not guaranteed)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": []test.Contains{
				test.Contains(`"name":"John"`),
				test.Contains(`"age":30`),
			},
		})
	})

	t.Run("StringWithSpecialCharsWithRawTrue", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract-special-chars-raw
    type: jq
    config:
      raw: true
    script: |
      {"message": "hello\nworld\ttab"}
    command: .message
    output: RESULT
`)

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "hello\nworld\ttab",
		})
	})
}
