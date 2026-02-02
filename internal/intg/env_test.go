package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestEnv(t *testing.T) {
	t.Parallel()

	t.Run("EnvVariables", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - TEST_ENV_1: "env_value_1"

params:
  - TEST_PARAM_1: ${TEST_ENV_1:0:3}_param

steps:
  - command: echo "${TEST_PARAM_1}"
    output: PARAM_OUTPUT
  - env:
      - STEP_ENV_1: "${TEST_PARAM_1}_step1"
    command: echo "${STEP_ENV_1}"
    output: STEP1_OUTPUT
  - env:
      - STEP_ENV_1: "${TEST_ENV_1:0:3}_step2"
    command: echo "${STEP_ENV_1}"
    output: STEP2_OUTPUT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"PARAM_OUTPUT": "env_param",
			"STEP1_OUTPUT": "env_param_step1",
			"STEP2_OUTPUT": "env_step2",
		})
	})

	t.Run("Derivatives", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
params:
  - UID: HBL01_22OCT2025_0536

steps:
  - name: step1
    command: echo $SEN
    env:
       - SEN: ${UID:0:5}
    output: STEP1_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP1_OUTPUT": "HBL01",
		})
	})

	t.Run("ShellFallbacks", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: default-env
    env:
      - OPTIONAL: ${UNSET_OPTIONAL:-default_value}
    command: echo "${OPTIONAL}"
    output: FALLBACK_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"FALLBACK_OUTPUT": "default_value",
		})
	})

	t.Run("StepOutputSubstrings", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
steps:
  - id: producer
    name: producer
    command: echo "HBL01_22OCT2025_0536"
    output: PRODUCER_OUTPUT
  - id: substring_validate
    name: substring-validate
    depends: producer
    command: |
      if [ "${producer.stdout:0:5}${producer.stdout:5}" = "${producer.stdout}" ]; then
        echo OK
      else
        echo FAIL
        exit 1
      fi
    output: SUBSTRING_VALIDATION
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"PRODUCER_OUTPUT":      "HBL01_22OCT2025_0536",
			"SUBSTRING_VALIDATION": "OK",
		})
	})
}
