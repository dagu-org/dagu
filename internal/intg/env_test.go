// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	t.Parallel()

	t.Run("EnvVariables", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - TEST_ENV_1: "env_value_1"

params:
  - TEST_PARAM_1: my_param

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
			"PARAM_OUTPUT": "my_param",
			"STEP1_OUTPUT": "my_param_step1",
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

	t.Run("DAGRunWorkDir", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
steps:
  - name: write-to-workdir
    command: echo "hello" > "${DAG_RUN_WORK_DIR}/test.txt"
  - name: read-from-workdir
    command: cat "${DAG_RUN_WORK_DIR}/test.txt"
    output: WORKDIR_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"WORKDIR_OUTPUT": "hello",
		})
	})

	t.Run("DAGRunWorkDirWithExplicitWorkingDir", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		explicitDir := t.TempDir()
		dag := th.DAG(t, `
working_dir: `+explicitDir+`
steps:
  - name: check-pwd
    command: pwd
    output: PWD_OUTPUT
  - name: write-to-workdir
    command: echo "from-workdir" > "${DAG_RUN_WORK_DIR}/data.txt"
  - name: read-from-workdir
    command: cat "${DAG_RUN_WORK_DIR}/data.txt"
    output: WORKDIR_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		// PWD should be the explicit working_dir, not the work dir
		// DAG_RUN_WORK_DIR should still be usable
		dag.AssertOutputs(t, map[string]any{
			"PWD_OUTPUT":     explicitDir,
			"WORKDIR_OUTPUT": "from-workdir",
		})
	})

	t.Run("EnvReferencesParams", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
params:
  data_dir: /tmp/foo
env:
  - FULL_PATH: "${data_dir}/output"
steps:
  - command: echo "${FULL_PATH}"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "/tmp/foo/output",
		})
	})

	t.Run("EnvReferencesParamsChained", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
params:
  base: /data
env:
  - DIR: "${base}/subdir"
  - FULL: "${DIR}/file.txt"
steps:
  - command: echo "${FULL}"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "/data/subdir/file.txt",
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

func TestSubDAGParamsReferencedInChildEnv(t *testing.T) {
	t.Parallel()

	th := test.SetupCommand(t)
	dagFile := th.CreateDAGFile(t, "subdag-env-from-params.yaml", `
name: subdag-env-parent
steps:
  - name: invoke-child
    call: subdag-env-child
    params: "data_dir=/mnt/data"

---
name: subdag-env-child
params:
  - name: data_dir
    type: string
    required: true
env:
  - OUTPUT_PATH: "${data_dir}/results"
steps:
  - name: check-env
    command: echo "${OUTPUT_PATH}"
    output: RESULT
`)

	runID := uuid.Must(uuid.NewV7()).String()
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        []string{"start", "--run-id", runID, dagFile},
		ExpectedOut: []string{"DAG run finished"},
	})

	rootRef := exec1.NewDAGRunRef("subdag-env-parent", runID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(th.Context, rootRef)
	require.NoError(t, err)

	parentStatus, err := parentAttempt.ReadStatus(th.Context)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, parentStatus.Status)
	require.Len(t, parentStatus.Nodes, 1)
	require.Len(t, parentStatus.Nodes[0].SubRuns, 1)

	subRunID := parentStatus.Nodes[0].SubRuns[0].DAGRunID
	subStatus, subOutputs := readSubAttemptStatusAndOutputs(t, th, rootRef, subRunID)
	require.Equal(t, core.Succeeded, subStatus.Status)

	require.Contains(t, subOutputs.Outputs, "result")
	assert.Equal(t, "/mnt/data/results", subOutputs.Outputs["result"])
}
