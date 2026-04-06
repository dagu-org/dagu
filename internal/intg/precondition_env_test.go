// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"testing"

	"github.com/dagucloud/dagu/internal/test"
)

func TestPreconditionWithDAGEnvVars(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, `
env:
  - DEV_PCENT: "90"
  - DEV_ALERT: "80"
steps:
  - name: check-threshold
    command: echo "alert triggered"
    output: RESULT
    preconditions:
      - condition: "test ${DEV_PCENT} -ge ${DEV_ALERT}"
`)
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "alert triggered",
	})
}

func TestPreconditionWithDAGEnvVarsNotMet(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, `
type: graph
env:
  - DEV_PCENT: "50"
  - DEV_ALERT: "80"
steps:
  - name: check-threshold
    command: echo "alert triggered"
    output: RESULT
    preconditions:
      - condition: "test ${DEV_PCENT} -ge ${DEV_ALERT}"
`)
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "",
	})
}

func TestDAGLevelPreconditionWithEnvVars(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, `
env:
  - ENABLED: "yes"
preconditions:
  - condition: "test ${ENABLED} = yes"
steps:
  - name: run
    command: echo "executed"
    output: RESULT
`)
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "executed",
	})
}

func TestPreconditionWithStepOutput(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, `
type: graph
steps:
  - name: produce
    command: echo "go"
    output: STEP_RESULT
  - name: consume
    command: echo "ran"
    output: FINAL
    preconditions:
      - condition: "test ${STEP_RESULT} = go"
    depends: produce
`)
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"STEP_RESULT": "go",
		"FINAL":       "ran",
	})
}
