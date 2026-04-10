// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomStepTypes_DAGLocalExecTemplate(t *testing.T) {
	t.Parallel()

	dag, err := LoadYAML(context.Background(), []byte(`
name: custom-step-local
step_types:
  greet:
    type: command
    description: Send a greeting
    input_schema:
      type: object
      additionalProperties: false
      required: [message]
      properties:
        message:
          type: string
        repeat:
          type: integer
          default: 3
    template:
      exec:
        command: /bin/echo
        args:
          - {$input: message}
          - {$input: repeat}
steps:
  - type: greet
    config:
      message: hello
`))
	require.NoError(t, err)
	require.Len(t, dag.Steps, 1)

	step := dag.Steps[0]
	assert.Equal(t, "greet_1", step.Name)
	assert.Equal(t, "direct", step.Shell)
	require.Len(t, step.Commands, 1)
	assert.Equal(t, "/bin/echo", step.Commands[0].Command)
	assert.Equal(t, []string{"hello", "3"}, step.Commands[0].Args)
	assert.Equal(t, "greet", step.ExecutorConfig.Metadata["custom_type"])
	assert.Equal(t, "Send a greeting", step.Description)
}

func TestCustomStepTypes_BaseConfigRegistry(t *testing.T) {
	t.Parallel()

	baseYAML := []byte(`
step_types:
  greet:
    type: command
    input_schema:
      type: object
      additionalProperties: false
      required: [message]
      properties:
        message:
          type: string
    template:
      exec:
        command: /bin/echo
        args:
          - {$input: message}
`)

	dag, err := LoadYAML(context.Background(), []byte(`
name: custom-step-base
steps:
  - type: greet
    config:
      message: hello-from-base
`), WithBaseConfigContent(baseYAML))
	require.NoError(t, err)
	require.Len(t, dag.Steps, 1)

	step := dag.Steps[0]
	assert.Equal(t, "greet_1", step.Name)
	require.Len(t, step.Commands, 1)
	assert.Equal(t, []string{"hello-from-base"}, step.Commands[0].Args)
	assert.Equal(t, "greet", step.ExecutorConfig.Metadata["custom_type"])
}

func TestCustomStepTypes_DuplicateNameAcrossScopes(t *testing.T) {
	t.Parallel()

	baseYAML := []byte(`
step_types:
  greet:
    type: command
    input_schema:
      type: object
      additionalProperties: false
      properties: {}
    template:
      exec:
        command: /bin/echo
`)

	_, err := LoadYAML(context.Background(), []byte(`
name: custom-step-duplicate
step_types:
  greet:
    type: command
    input_schema:
      type: object
      additionalProperties: false
      properties: {}
    template:
      exec:
        command: /bin/echo
steps:
  - type: greet
`), WithBaseConfigContent(baseYAML))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate custom step type "greet"`)
}

func TestCustomStepTypes_RejectsForbiddenCallSiteFields(t *testing.T) {
	t.Parallel()

	_, err := LoadYAML(context.Background(), []byte(`
name: custom-step-forbidden
step_types:
  greet:
    type: command
    input_schema:
      type: object
      additionalProperties: false
      required: [message]
      properties:
        message:
          type: string
    template:
      exec:
        command: /bin/echo
        args:
          - {$input: message}
steps:
  - type: greet
    command: echo should-fail
    config:
      message: hello
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `field "command" is not allowed`)
}

func TestCustomStepTypes_HandlerSupport(t *testing.T) {
	t.Parallel()

	dag, err := LoadYAML(context.Background(), []byte(`
name: custom-step-handler
step_types:
  greet:
    type: command
    input_schema:
      type: object
      additionalProperties: false
      required: [message]
      properties:
        message:
          type: string
    template:
      exec:
        command: /bin/echo
        args:
          - {$input: message}
handler_on:
  success:
    type: greet
    config:
      message: handler-ok
steps:
  - command: echo run
`))
	require.NoError(t, err)
	require.NotNil(t, dag.HandlerOn.Success)
	assert.Equal(t, "onSuccess", dag.HandlerOn.Success.Name)
	assert.Equal(t, "direct", dag.HandlerOn.Success.Shell)
	assert.Equal(t, "greet", dag.HandlerOn.Success.ExecutorConfig.Metadata["custom_type"])
	require.Len(t, dag.HandlerOn.Success.Commands, 1)
	assert.Equal(t, []string{"handler-ok"}, dag.HandlerOn.Success.Commands[0].Args)
}

func TestStepExec_BuildsDirectCommand(t *testing.T) {
	t.Parallel()

	dag, err := LoadYAML(context.Background(), []byte(`
name: exec-step
steps:
  - exec:
      command: /bin/echo
      args: [hello, 3, true]
`))
	require.NoError(t, err)
	require.Len(t, dag.Steps, 1)

	step := dag.Steps[0]
	assert.Equal(t, "cmd_1", step.Name)
	assert.Equal(t, "direct", step.Shell)
	require.Len(t, step.Commands, 1)
	assert.Equal(t, "/bin/echo", step.Commands[0].Command)
	assert.Equal(t, []string{"hello", "3", "true"}, step.Commands[0].Args)
}

func TestStepExec_RejectsCommandConflict(t *testing.T) {
	t.Parallel()

	_, err := LoadYAML(context.Background(), []byte(`
name: exec-step-invalid
steps:
  - command: echo hello
    exec:
      command: /bin/echo
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec cannot be used together with command")
}
