// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/dagucloud/dagu/internal/test"
)

func numericPreconditionCommand(left, operator, right string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("if ([int]%s %s [int]%s) { exit 0 } else { exit 1 }", left, operator, right)
	}
	return fmt.Sprintf("test %s %s %s", left, operator, right)
}

func stringPreconditionCommand(left, right string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("if (%q -eq %q) { exit 0 } else { exit 1 }", left, right)
	}
	return fmt.Sprintf("test %s = %s", left, right)
}

func TestPreconditionWithDAGEnvVars(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`
env:
  - DEV_PCENT: "90"
  - DEV_ALERT: "80"
steps:
  - name: check-threshold
    command: echo "alert triggered"
    output: RESULT
    preconditions:
      - condition: %q
`, numericPreconditionCommand("${DEV_PCENT}", "-ge", "${DEV_ALERT}")))
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "alert triggered",
	})
}

func TestPreconditionWithDAGEnvVarsNotMet(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`
type: graph
env:
  - DEV_PCENT: "50"
  - DEV_ALERT: "80"
steps:
  - name: check-threshold
    command: echo "alert triggered"
    output: RESULT
    preconditions:
      - condition: %q
`, numericPreconditionCommand("${DEV_PCENT}", "-ge", "${DEV_ALERT}")))
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "",
	})
}

func TestDAGLevelPreconditionWithEnvVars(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`
env:
  - ENABLED: "yes"
preconditions:
  - condition: %q
steps:
  - name: run
    command: echo "executed"
    output: RESULT
`, stringPreconditionCommand("${ENABLED}", "yes")))
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"RESULT": "executed",
	})
}

func TestPreconditionWithStepOutput(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`
type: graph
steps:
  - name: produce
    command: echo "go"
    output: STEP_RESULT
  - name: consume
    command: echo "ran"
    output: FINAL
    preconditions:
      - condition: %q
    depends: produce
`, stringPreconditionCommand("${STEP_RESULT}", "go")))
	agent := dag.Agent()
	agent.RunSuccess(t)
	dag.AssertOutputs(t, map[string]any{
		"STEP_RESULT": "go",
		"FINAL":       "ran",
	})
}
