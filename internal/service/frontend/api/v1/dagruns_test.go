// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime/transform"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func dagRunEventuallyTimeout(base time.Duration) time.Duration {
	if runtime.GOOS == "windows" {
		return base * 24
	}
	return base
}

func dagRunSyncTimeoutSeconds() int {
	if runtime.GOOS == "windows" {
		return 120
	}
	return 30
}

func waitForDAGRunStatus(
	t *testing.T,
	server test.Server,
	dagName string,
	dagRunID string,
	timeout time.Duration,
	predicate func(*exec.DAGRunStatus) bool,
) *exec.DAGRunStatus {
	t.Helper()

	dag := &core.DAG{Name: dagName}
	var status *exec.DAGRunStatus
	require.Eventually(t, func() bool {
		current, err := server.DAGRunMgr.GetCurrentStatus(server.Context, dag, dagRunID)
		if err != nil || current == nil {
			return false
		}
		status = current
		return predicate(current)
	}, dagRunEventuallyTimeout(timeout), 200*time.Millisecond)

	return status
}

func TestGetDAGRunSpec(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo spec_test"`

	// Create a new DAG
	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "spec_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start a DAG run
	startResp := server.Client().Post("/api/v1/dags/spec_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	waitForDAGRunStatus(t, server, "spec_test_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})

	// Fetch the DAG spec for the DAG run
	specResp := server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/spec", "spec_test_dag", startBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var specBody api.GetDAGRunSpec200JSONResponse
	specResp.Unmarshal(t, &specBody)
	require.NotEmpty(t, specBody.Spec)
	require.Contains(t, specBody.Spec, "echo spec_test")

	// Test 404 for non-existent DAG
	_ = server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/spec", "non_existent_dag", startBody.DagRunId),
	).ExpectStatus(http.StatusNotFound).Send(t)
}

func TestGetDAGRunSpecInline(t *testing.T) {
	server := test.SetupServer(t)

	inlineSpec := `steps:
  - name: inline_step
    command: "echo inline_dag_test"`

	name := "inline_spec_dag"

	// Execute an inline DAG run
	execResp := server.Client().Post("/api/v1/dag-runs", api.ExecuteDAGRunFromSpecJSONRequestBody{
		Spec: inlineSpec,
		Name: &name,
	}).ExpectStatus(http.StatusOK).Send(t)

	var execBody api.ExecuteDAGRunFromSpec200JSONResponse
	execResp.Unmarshal(t, &execBody)
	require.NotEmpty(t, execBody.DagRunId)

	specBody := requireDAGRunSpec(t, server, name, execBody.DagRunId)
	require.Contains(t, specBody.Spec, "echo inline_dag_test")
}

func TestGetDAGRunSpecInlineStartWithTagsDoesNotPatchSpec(t *testing.T) {
	server := test.SetupServer(t)

	inlineSpec := `steps:
  - name: inline_step
    command: "echo inline_start_tags"`
	name := "inline_spec_start_tags"
	tags := []string{"env=prod", "team=backend"}

	execResp := server.Client().Post("/api/v1/dag-runs", api.ExecuteDAGRunFromSpecJSONRequestBody{
		Spec: inlineSpec,
		Name: &name,
		Tags: &tags,
	}).ExpectStatus(http.StatusOK).Send(t)

	var execBody api.ExecuteDAGRunFromSpec200JSONResponse
	execResp.Unmarshal(t, &execBody)
	require.NotEmpty(t, execBody.DagRunId)

	specBody := requireDAGRunSpec(t, server, name, execBody.DagRunId)
	require.Contains(t, specBody.Spec, "echo inline_start_tags")
	require.NotContains(t, specBody.Spec, "tags:")
	require.NotContains(t, specBody.Spec, "env=prod")
	require.NotContains(t, specBody.Spec, "team=backend")
}

func requireDAGRunSpec(t *testing.T, server test.Server, dagName, dagRunID string) api.GetDAGRunSpec200JSONResponse {
	t.Helper()

	var specBody api.GetDAGRunSpec200JSONResponse
	require.Eventually(t, func() bool {
		specResp := server.Client().Get(
			fmt.Sprintf("/api/v1/dag-runs/%s/%s/spec", dagName, dagRunID),
		).Send(t)
		if specResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		specResp.Unmarshal(t, &specBody)
		return specBody.Spec != ""
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	return specBody
}

func TestGetDAGRunSpecInlineEnqueueWithTagsPatchesSpec(t *testing.T) {
	server := test.SetupServer(t)

	inlineSpec := `steps:
  - name: inline_step
    command: "echo inline_enqueue_tags"`
	name := "inline_enqueue_tags"
	tags := []string{"env=prod", "team=backend"}

	enqResp := server.Client().Post("/api/v1/dag-runs/enqueue", api.EnqueueDAGRunFromSpecJSONRequestBody{
		Spec: inlineSpec,
		Name: &name,
		Tags: &tags,
	}).ExpectStatus(http.StatusOK).Send(t)

	var enqBody api.EnqueueDAGRunFromSpec200JSONResponse
	enqResp.Unmarshal(t, &enqBody)
	require.NotEmpty(t, enqBody.DagRunId)

	require.Eventually(t, func() bool {
		statusResp := server.Client().
			Get(fmt.Sprintf("/api/v1/dag-runs/%s/%s", name, enqBody.DagRunId)).
			Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		status := dagRunStatus.DagRunDetails.Status
		return status == api.Status(core.Queued) ||
			status == api.Status(core.Running) ||
			status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	specResp := server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/spec", name, enqBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var specBody api.GetDAGRunSpec200JSONResponse
	specResp.Unmarshal(t, &specBody)
	require.Contains(t, specBody.Spec, "echo inline_enqueue_tags")
	require.Contains(t, specBody.Spec, "tags:")
	require.Contains(t, specBody.Spec, "env=prod")
	require.Contains(t, specBody.Spec, "team=backend")
}

func TestGetDAGRunSpecFileEnqueueWithTagsDoesNotPatchSpec(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo file_enqueue_tags"`
	dagName := "file_enqueue_tags"
	tags := []string{"env=staging", "priority=low"}

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	enqResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dags/%s/enqueue", dagName),
		api.EnqueueDAGDAGRunJSONRequestBody{Tags: &tags},
	).ExpectStatus(http.StatusOK).Send(t)

	var enqBody api.EnqueueDAGDAGRun200JSONResponse
	enqResp.Unmarshal(t, &enqBody)
	require.NotEmpty(t, enqBody.DagRunId)

	require.Eventually(t, func() bool {
		statusResp := server.Client().
			Get(fmt.Sprintf("/api/v1/dag-runs/%s/%s", dagName, enqBody.DagRunId)).
			Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		status := dagRunStatus.DagRunDetails.Status
		return status == api.Status(core.Queued) ||
			status == api.Status(core.Running) ||
			status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	specResp := server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/spec", dagName, enqBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var specBody api.GetDAGRunSpec200JSONResponse
	specResp.Unmarshal(t, &specBody)
	require.Contains(t, specBody.Spec, "echo file_enqueue_tags")
	require.NotContains(t, specBody.Spec, "tags:")
	require.NotContains(t, specBody.Spec, "env=staging")
	require.NotContains(t, specBody.Spec, "priority=low")
}

func TestGetSubDAGRunSpec(t *testing.T) {
	server := test.SetupServer(t)
	childCommand := test.PortableOutputCommand("subdag-spec")

	// Create a parent DAG with an inline sub-DAG definition
	dagSpec := fmt.Sprintf(`steps:
  - name: call_child
    call: child_dag
    params: "MSG=hello"

---

name: child_dag
params:
  - MSG
steps:
  - name: echo_message
    command: %q`, childCommand)

	// Create the parent DAG
	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "parent_dag_for_subdag_spec",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the parent DAG
	startResp := server.Client().Post("/api/v1/dags/parent_dag_for_subdag_spec/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	status := waitForDAGRunStatus(t, server, "parent_dag_for_subdag_spec", startBody.DagRunId, 30*time.Second,
		func(status *exec.DAGRunStatus) bool {
			return status.Status == core.Succeeded &&
				len(status.Nodes) == 1 &&
				len(status.Nodes[0].SubRuns) == 1
		},
	)
	require.Len(t, status.Nodes, 1, "Expected 1 node (the call_child step)")

	callNode := status.Nodes[0]
	require.Equal(t, "call_child", callNode.Step.Name)
	subDAGRunID := callNode.SubRuns[0].DAGRunID

	// Test 1: Fetch the sub-DAG spec successfully
	subSpecResp := server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, subDAGRunID),
	).ExpectStatus(http.StatusOK).Send(t)

	var subSpecBody api.GetSubDAGRunSpec200JSONResponse
	subSpecResp.Unmarshal(t, &subSpecBody)
	require.NotEmpty(t, subSpecBody.Spec, "Sub-DAG spec should not be empty")
	require.Contains(t, subSpecBody.Spec, "child_dag", "Spec should contain child_dag name")
	require.Contains(t, subSpecBody.Spec, "echo_message", "Spec should contain echo_message step")
	require.Contains(t, subSpecBody.Spec, "subdag-spec", "Spec should contain the command")

	// Test 2: 404 for non-existent sub-DAG run ID
	_ = server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, "non_existent_sub_dag_id"),
	).ExpectStatus(http.StatusNotFound).Send(t)

	// Test 3: 404 for non-existent parent DAG
	_ = server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/non_existent_dag/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, subDAGRunID),
	).ExpectStatus(http.StatusNotFound).Send(t)

	// Test 4: 404 for non-existent parent DAG run ID
	_ = server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			"non_existent_run_id", subDAGRunID),
	).ExpectStatus(http.StatusNotFound).Send(t)
}

func TestApproveDAGRunStep(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo approved"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v1/dags/approval_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	waitForDAGRunStatus(t, server, "approval_test_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Waiting
	})

	// Approve the wait step
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/approval_test_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.Equal(t, startBody.DagRunId, approveBody.DagRunId)
	require.Equal(t, "wait-step", approveBody.StepName)
	require.True(t, approveBody.Resumed)

	// Wait for DAG to complete
	waitForDAGRunStatus(t, server, "approval_test_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})
}

func TestApproveDAGRunStepWithInputs(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := fmt.Sprintf(`type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please provide reason"
      input:
        - reason
        - approver
      required:
        - reason
  - name: after-wait
    depends: [wait-step]
    command: %q`, test.PortableEnvOutputCommand("reason", "approver"))

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_inputs_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v1/dags/approval_inputs_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	waitForDAGRunStatus(t, server, "approval_inputs_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Waiting
	})

	// Approve with inputs
	inputs := map[string]string{
		"reason":   "testing",
		"approver": "test-user",
	}
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/approval_inputs_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{Inputs: &inputs},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.True(t, approveBody.Resumed)

	// Wait for DAG to complete
	status := waitForDAGRunStatus(t, server, "approval_inputs_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})
	require.Len(t, status.Nodes, 2)

	var waitNode, afterWaitNode *exec.Node
	for _, node := range status.Nodes {
		switch node.Step.Name {
		case "wait-step":
			waitNode = node
		case "after-wait":
			afterWaitNode = node
		}
	}
	require.NotNil(t, waitNode)
	require.NotNil(t, afterWaitNode)
	require.Equal(t, inputs, waitNode.ApprovalInputs)

	stdout, err := os.ReadFile(afterWaitNode.Stdout)
	require.NoError(t, err)
	require.Equal(t, "testing|test-user", strings.TrimSpace(string(stdout)))
}

func TestApproveDAGRunStepMissingRequired(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please provide reason"
      input:
        - reason
      required:
        - reason
  - name: after-wait
    depends: [wait-step]
    command: "echo done"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_required_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v1/dags/approval_required_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	// Wait for DAG to enter Wait status
	waitForDAGRunStatus(t, server, "approval_required_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Waiting
	})

	// Try to approve without required input - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/approval_required_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestApproveDAGRunStepNotWaiting(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo done"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "no_wait_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start and wait for completion
	startResp := server.Client().Post("/api/v1/dags/no_wait_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	waitForDAGRunStatus(t, server, "no_wait_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})

	// Try to approve a step that's not waiting - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/no_wait_dag/%s/steps/main/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestRejectDAGRunStep(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo should not run"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "rejection_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v1/dags/rejection_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	waitForDAGRunStatus(t, server, "rejection_test_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Waiting
	})

	// Reject the wait step
	reason := "test rejection reason"
	rejectResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/rejection_test_dag/%s/steps/wait-step/reject", startBody.DagRunId),
		api.RejectStepRequest{Reason: &reason},
	).ExpectStatus(http.StatusOK).Send(t)

	var rejectBody api.RejectDAGRunStep200JSONResponse
	rejectResp.Unmarshal(t, &rejectBody)
	require.Equal(t, startBody.DagRunId, rejectBody.DagRunId)
	require.Equal(t, "wait-step", rejectBody.StepName)

	// Verify DAG status is Rejected
	waitForDAGRunStatus(t, server, "rejection_test_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Rejected
	})
}

func TestRejectDAGRunStepNotWaiting(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo done"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reject_not_waiting_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v1/dags/reject_not_waiting_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	waitForDAGRunStatus(t, server, "reject_not_waiting_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})

	// Try to reject a step that's not waiting - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/reject_not_waiting_dag/%s/steps/main/reject", startBody.DagRunId),
		api.RejectStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestRescheduleDAGRun(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "reschedule_dag", MaxActiveRuns: 1},
		}
	}))

	dagSpec := `steps:
  - name: main
    command: "echo reschedule"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v1/dags/reschedule_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	waitForDAGRunStatus(t, server, "reschedule_dag", startBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})

	rescheduleResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/reschedule", "reschedule_dag", startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var rescheduleBody api.RescheduleDAGRun200JSONResponse
	rescheduleResp.Unmarshal(t, &rescheduleBody)
	require.NotEmpty(t, rescheduleBody.DagRunId)
	require.True(t, rescheduleBody.Queued)

	test.ProcessQueuedInlineRun(t, server, "reschedule_dag")

	waitForDAGRunStatus(t, server, "reschedule_dag", rescheduleBody.DagRunId, 10*time.Second, func(status *exec.DAGRunStatus) bool {
		return status.Status == core.Succeeded
	})
}

func TestRescheduleDAGRunResolvesLatest(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "reschedule_latest_dag", MaxActiveRuns: 1},
		}
	}))

	dagSpec := `steps:
  - name: main
    command: "echo reschedule latest"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_latest_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v1/dags/reschedule_latest_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v1/dags/reschedule_latest_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	rescheduleResp := server.Client().Post(
		"/api/v1/dag-runs/reschedule_latest_dag/latest/reschedule",
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var rescheduleBody api.RescheduleDAGRun200JSONResponse
	rescheduleResp.Unmarshal(t, &rescheduleBody)
	require.NotEmpty(t, rescheduleBody.DagRunId)
	require.True(t, rescheduleBody.Queued)

	test.ProcessQueuedInlineRun(t, server, "reschedule_latest_dag")

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v1/dags/reschedule_latest_dag/dag-runs/%s", rescheduleBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)
}

func TestRescheduleDAGRunFromInlineStartUsesPersistedSnapshot(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "inline_reschedule_start", MaxActiveRuns: 1},
		}
	}))

	runID, dagLocation := test.CreateInlineDAGRunForReschedule(t, server, "inline_reschedule_start", false)
	require.NoFileExists(t, dagLocation)
	assertRescheduleSpecSourceFlag(t, server, "inline_reschedule_start", runID, false)

	rescheduledRunID := rescheduleInlineDAGRun(t, server, "inline_reschedule_start", runID)
	test.ProcessQueuedInlineRun(t, server, "inline_reschedule_start")
	test.AssertInlineRescheduledRunParams(t, server, "inline_reschedule_start", rescheduledRunID)
}

func TestRescheduleDAGRunFromInlineEnqueueUsesPersistedSnapshot(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "inline_reschedule_enqueue", MaxActiveRuns: 1},
		}
	}))

	runID, dagLocation := test.CreateInlineDAGRunForReschedule(t, server, "inline_reschedule_enqueue", true)
	require.NoFileExists(t, dagLocation)
	assertRescheduleSpecSourceFlag(t, server, "inline_reschedule_enqueue", runID, false)

	rescheduledRunID := rescheduleInlineDAGRun(t, server, "inline_reschedule_enqueue", runID)
	test.ProcessQueuedInlineRun(t, server, "inline_reschedule_enqueue")
	test.AssertInlineRescheduledRunParams(t, server, "inline_reschedule_enqueue", rescheduledRunID)
}

func TestRescheduleDAGRunCanUseCurrentDAGFile(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "reschedule_use_current_file", MaxActiveRuns: 1},
		}
	}))

	dagName := "reschedule_use_current_file"
	initialSpec := `queue: reschedule_use_current_file
steps:
  - name: main
    command: echo stored snapshot`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &initialSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post(
		fmt.Sprintf("/api/v1/dags/%s/start", dagName),
		api.ExecuteDAGJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	currentSpec := `queue: reschedule_use_current_file
steps:
  - name: main
    command: echo current file`
	dagPath := filepath.Join(server.Config.Paths.DAGsDir, dagName+".yaml")
	assertRescheduleSpecSourceFlag(t, server, dagName, startBody.DagRunId, true)
	originalAttempt, originalDAG := test.WaitForAttemptSnapshotWithDAG(t, server, dagName, startBody.DagRunId)
	require.NotNil(t, originalAttempt)
	require.Equal(t, dagPath, originalDAG.SourceFile)
	require.NoError(t, os.WriteFile(dagPath, []byte(currentSpec), 0o600))
	useCurrentDagFile := true

	resp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/reschedule", dagName, startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{UseCurrentDagFile: &useCurrentDagFile},
	).ExpectStatus(http.StatusOK).Send(t)

	var body api.RescheduleDAGRun200JSONResponse
	resp.Unmarshal(t, &body)
	require.NotEmpty(t, body.DagRunId)
	require.True(t, body.Queued)

	test.ProcessQueuedInlineRun(t, server, dagName)

	attempt, dag := test.WaitForAttemptSnapshotWithDAG(t, server, dagName, body.DagRunId)
	require.Contains(t, string(dag.YamlData), "echo current file")
	require.Equal(t, dagPath, dag.SourceFile)

	require.Eventually(t, func() bool {
		status, err := attempt.ReadStatus(server.Context)
		return err == nil && status.Status == core.Succeeded
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)
}

func TestRescheduleDAGRunRequiresQueuesEnabled(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = false
		cfg.Queues.Config = nil
	}))

	dagSpec := `steps:
  - name: main
    command: "echo reschedule disabled"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_requires_queue_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post(
		"/api/v1/dags/reschedule_requires_queue_dag/start",
		api.ExecuteDAGJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v1/dags/reschedule_requires_queue_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)

	server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/reschedule", "reschedule_requires_queue_dag", startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestRetryDAGRunQueuesRetryForQueuedDAGs(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Queues.Enabled = true
		cfg.Queues.Config = []config.QueueConfig{
			{Name: "single-retry-queue", MaxActiveRuns: 1},
		}
	}))

	dag := server.DAG(t, `
name: single_retry_queue_dag
queue: single-retry-queue
steps:
  - name: main
    command: echo queued retry
`)

	seedLatestDAGRunStatus(t, server, dag.DAG, "queued-run", core.Failed, seedDAGRunStatusOptions{
		errorText: "queued run failed",
	})

	server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/retry", dag.Name, "queued-run"),
		api.RetryDAGRunJSONRequestBody{DagRunId: "queued-run"},
	).ExpectStatus(http.StatusOK).Send(t)

	attempt, err := server.DAGRunStore.FindAttempt(server.Context, exec.NewDAGRunRef(dag.Name, "queued-run"))
	require.NoError(t, err)

	status, err := attempt.ReadStatus(server.Context)
	require.NoError(t, err)
	require.Equal(t, core.Queued, status.Status)
	require.Equal(t, core.TriggerTypeRetry, status.TriggerType)
}

func TestRetryDAGRunStartsLocalRetrySubprocess(t *testing.T) {
	server := test.SetupServer(t)

	retryCommand := `
if [ -f "$DAG_RUN_LOG_FILE.marker" ]; then
  echo local retry
else
  touch "$DAG_RUN_LOG_FILE.marker"
  exit 1
fi
`
	if runtime.GOOS == "windows" {
		retryCommand = `
if (Test-Path "$env:DAG_RUN_LOG_FILE.marker") {
  Write-Output "local retry"
} else {
  New-Item -ItemType File -Path "$env:DAG_RUN_LOG_FILE.marker" -Force | Out-Null
  exit 1
}
`
	}

	dagSpec := fmt.Sprintf(`
steps:
  - name: main
    command: |
%s
`, indentCommandBlock(retryCommand, 6))

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "single_retry_local_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post(
		"/api/v1/dags/single_retry_local_dag/start",
		api.ExecuteDAGJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		resp := server.Client().Get(
			fmt.Sprintf("/api/v1/dag-runs/%s/%s", "single_retry_local_dag", startBody.DagRunId),
		).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var details api.GetDAGRunDetails200JSONResponse
		resp.Unmarshal(t, &details)
		return details.DagRunDetails.Status == api.Status(core.Failed)
	}, dagRunEventuallyTimeout(15*time.Second), 200*time.Millisecond)

	server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/retry", "single_retry_local_dag", startBody.DagRunId),
		api.RetryDAGRunJSONRequestBody{DagRunId: startBody.DagRunId},
	).ExpectStatus(http.StatusOK).Send(t)

	require.Eventually(t, func() bool {
		resp := server.Client().Get(
			fmt.Sprintf("/api/v1/dag-runs/%s/%s", "single_retry_local_dag", startBody.DagRunId),
		).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var details api.GetDAGRunDetails200JSONResponse
		resp.Unmarshal(t, &details)
		return details.DagRunDetails.Status == api.Status(core.Succeeded)
	}, dagRunEventuallyTimeout(15*time.Second), 200*time.Millisecond)
}

func TestTerminateDAGRunCancelsFailedAutoRetryPendingRun(t *testing.T) {
	server := test.SetupServer(t)

	dag := server.DAG(t, `
name: cancel_failed_retry_dag
retry_policy:
  limit: 3
  interval_sec: 60
steps:
  - name: main
    command: "echo fail"
`)

	ref := seedLatestDAGRunStatus(
		t,
		server,
		dag.DAG,
		"cancel-failed-run",
		core.Failed,
		seedDAGRunStatusOptions{
			autoRetryCount: 1,
			errorText:      "step failed",
		},
	)

	_ = server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/stop", dag.Name, ref.ID),
		nil,
	).ExpectStatus(http.StatusOK).Send(t)

	persisted := test.ReadRunStatus(server.Context, t, server.DAGRunStore, ref)
	require.Equal(t, core.Aborted, persisted.Status)
	require.Equal(t, 1, persisted.AutoRetryCount)
	require.Equal(t, 3, persisted.AutoRetryLimit)
	require.Equal(t, "step failed", persisted.Error)
	require.Len(t, persisted.Nodes, 1)
	require.Equal(t, core.NodeFailed, persisted.Nodes[0].Status)

	resp := server.Client().Get(fmt.Sprintf("/api/v1/dag-runs/%s/%s", dag.Name, ref.ID)).
		ExpectStatus(http.StatusOK).
		Send(t)

	var body api.GetDAGRunDetails200JSONResponse
	resp.Unmarshal(t, &body)
	require.Equal(t, api.Status(core.Aborted), body.DagRunDetails.Status)
}

func TestTerminateDAGRunRejectsFailedRunWithoutPendingAutoRetry(t *testing.T) {
	server := test.SetupServer(t)

	dag := server.DAG(t, `
name: cancel_failed_retry_exhausted_dag
retry_policy:
  limit: 3
  interval_sec: 60
steps:
  - name: main
    command: "echo fail"
`)

	ref := seedLatestDAGRunStatus(
		t,
		server,
		dag.DAG,
		"cancel-failed-exhausted-run",
		core.Failed,
		seedDAGRunStatusOptions{
			autoRetryCount: 3,
			errorText:      "still failed",
		},
	)

	resp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/stop", dag.Name, ref.ID),
		nil,
	).ExpectStatus(http.StatusBadRequest).Send(t)

	var errBody api.Error
	resp.Unmarshal(t, &errBody)
	require.Equal(t, api.ErrorCodeBadRequest, errBody.Code)
	require.Contains(t, errBody.Message, "not pending auto-retry")

	persisted := test.ReadRunStatus(server.Context, t, server.DAGRunStore, ref)
	require.Equal(t, core.Failed, persisted.Status)
	require.Equal(t, 3, persisted.AutoRetryCount)
}

func TestExecuteDAGSync(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: echo-step
    command: "echo hello sync"`

	// Create a new DAG
	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously with timeout
	timeout := dagRunSyncTimeoutSeconds()
	syncResp := server.Client().Post("/api/v1/dags/sync_test_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
		Timeout: timeout,
	}).ExpectStatus(http.StatusOK).Send(t)

	var syncBody api.ExecuteDAGSync200JSONResponse
	syncResp.Unmarshal(t, &syncBody)

	// Verify the response contains full DAGRunDetails
	require.NotEmpty(t, syncBody.DagRun.DagRunId)
	require.Equal(t, "sync_test_dag", syncBody.DagRun.Name)
	require.Equal(t, api.Status(core.Succeeded), syncBody.DagRun.Status)
	require.Equal(t, api.StatusLabel("succeeded"), syncBody.DagRun.StatusLabel)
	require.NotNil(t, syncBody.DagRun.Nodes)
	require.Len(t, syncBody.DagRun.Nodes, 1)
	require.Equal(t, "echo-step", syncBody.DagRun.Nodes[0].Step.Name)
}

func TestExecuteDAGSyncTimeout(t *testing.T) {
	server := test.SetupServer(t)

	// Create a DAG with a step that takes longer than the timeout
	dagSpec := `steps:
  - name: slow-step
    command: "sleep 10"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_timeout_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously with a very short timeout (1 second)
	timeout := 1
	syncResp := server.Client().Post("/api/v1/dags/sync_timeout_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
		Timeout: timeout,
	}).ExpectStatus(http.StatusRequestTimeout).Send(t)

	var errBody api.TimeoutError
	syncResp.Unmarshal(t, &errBody)
	require.Equal(t, api.ErrorCodeTimeout, errBody.Code)
	require.Contains(t, errBody.Message, "timeout")
	require.Contains(t, errBody.Message, "DAG run continues in background")
	require.NotEmpty(t, errBody.DagRunId, "408 response should include dagRunId for tracking")
}

func TestExecuteDAGSyncWithWaitingStatus(t *testing.T) {
	server := test.SetupServer(t)

	// Create a DAG with approval step that will wait for approval
	dagSpec := `steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Approve this"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_waiting_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously - should return when DAG reaches waiting status
	timeout := 30
	syncResp := server.Client().Post("/api/v1/dags/sync_waiting_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
		Timeout: timeout,
	}).ExpectStatus(http.StatusOK).Send(t)

	var syncBody api.ExecuteDAGSync200JSONResponse
	syncResp.Unmarshal(t, &syncBody)

	// Should return with waiting status (not timeout)
	require.NotEmpty(t, syncBody.DagRun.DagRunId)
	require.Equal(t, api.Status(core.Waiting), syncBody.DagRun.Status)
	require.Equal(t, api.StatusLabel("waiting"), syncBody.DagRun.StatusLabel)
}

type seedDAGRunStatusOptions struct {
	autoRetryCount int
	errorText      string
	parentRef      exec.DAGRunRef
	paramsList     []string
}

func seedLatestDAGRunStatus(
	t *testing.T,
	server test.Server,
	dag *core.DAG,
	dagRunID string,
	status core.Status,
	opts seedDAGRunStatusOptions,
) exec.DAGRunRef {
	t.Helper()

	attempt, err := server.DAGRunStore.CreateAttempt(
		server.Context,
		dag,
		time.Now().Add(-2*time.Minute),
		dagRunID,
		exec.NewDAGRunAttemptOptions{},
	)
	require.NoError(t, err)

	ref := exec.NewDAGRunRef(dag.Name, dagRunID)
	dagRunStatus := transform.NewStatusBuilder(dag).Create(
		dagRunID,
		status,
		0,
		time.Now().Add(-2*time.Minute),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(ref, opts.parentRef),
		transform.WithFinishedAt(time.Now().Add(-time.Minute)),
		transform.WithAutoRetryCount(opts.autoRetryCount),
		transform.WithError(opts.errorText),
	)
	if len(opts.paramsList) > 0 {
		dagRunStatus.ParamsList = append([]string(nil), opts.paramsList...)
	}
	if len(dagRunStatus.Nodes) > 0 && status == core.Failed {
		dagRunStatus.Nodes[0].Status = core.NodeFailed
		dagRunStatus.Nodes[0].FinishedAt = exec.FormatTime(time.Now().Add(-time.Minute))
		dagRunStatus.Nodes[0].Error = opts.errorText
	}

	require.NoError(t, attempt.Open(server.Context))
	require.NoError(t, attempt.Write(server.Context, dagRunStatus))
	require.NoError(t, attempt.Close(server.Context))

	return ref
}

func indentCommandBlock(command string, spaces int) string {
	trimmed := strings.Trim(command, "\n")
	if trimmed == "" {
		return ""
	}

	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(trimmed, "\n")
	return prefix + strings.Join(lines, "\n"+prefix)
}

func rescheduleInlineDAGRun(t *testing.T, server test.Server, dagName, dagRunID string) string {
	t.Helper()

	resp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/reschedule", dagName, dagRunID),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var body api.RescheduleDAGRun200JSONResponse
	resp.Unmarshal(t, &body)
	require.NotEmpty(t, body.DagRunId)
	require.True(t, body.Queued)
	return body.DagRunId
}

func assertRescheduleSpecSourceFlag(t *testing.T, server test.Server, dagName, dagRunID string, want bool) {
	t.Helper()

	resp := server.Client().Get(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s", dagName, dagRunID),
	).ExpectStatus(http.StatusOK).Send(t)

	var body api.GetDAGRunDetails200JSONResponse
	resp.Unmarshal(t, &body)
	got := body.DagRunDetails.SpecFromFile != nil && *body.DagRunDetails.SpecFromFile
	require.Equal(t, want, got)
}

func TestExecuteDAGSyncSingleton(t *testing.T) {
	server := test.SetupServer(t)

	// Create a DAG with a slow step
	dagSpec := `steps:
  - name: slow-step
    command: "sleep 5"`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_singleton_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG asynchronously first
	startResp := server.Client().Post("/api/v1/dags/sync_singleton_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Try to start another sync execution with singleton mode - should fail with 409
	singleton := true
	timeout := 30
	_ = server.Client().Post("/api/v1/dags/sync_singleton_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
		Timeout:   timeout,
		Singleton: &singleton,
	}).ExpectStatus(http.StatusConflict).Send(t)
}

func TestListDAGRunsFilterByTags(t *testing.T) {
	server := test.SetupServer(t)

	// Create DAGs with different tags
	dagSpecProd := `tags:
  - prod
  - critical
steps:
  - name: main
    command: "echo prod-critical"`

	dagSpecDev := `tags:
  - dev
  - critical
steps:
  - name: main
    command: "echo dev-critical"`

	dagSpecTest := `tags:
  - test
steps:
  - name: main
    command: "echo test-only"`

	// Create the DAGs
	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_prod",
		Spec: &dagSpecProd,
	}).ExpectStatus(http.StatusCreated).Send(t)

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_dev",
		Spec: &dagSpecDev,
	}).ExpectStatus(http.StatusCreated).Send(t)

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_test",
		Spec: &dagSpecTest,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start DAG runs for each
	var prodRunId, devRunId, testRunId string

	startResp := server.Client().Post("/api/v1/dags/tag_filter_prod/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)
	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	prodRunId = startBody.DagRunId

	startResp = server.Client().Post("/api/v1/dags/tag_filter_dev/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)
	startResp.Unmarshal(t, &startBody)
	devRunId = startBody.DagRunId

	startResp = server.Client().Post("/api/v1/dags/tag_filter_test/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)
	startResp.Unmarshal(t, &startBody)
	testRunId = startBody.DagRunId

	// Wait for all runs to complete
	for _, pair := range []struct {
		name  string
		runId string
	}{
		{"tag_filter_prod", prodRunId},
		{"tag_filter_dev", devRunId},
		{"tag_filter_test", testRunId},
	} {
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", pair.name, pair.runId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, dagRunEventuallyTimeout(10*time.Second), 200*time.Millisecond)
	}

	// Test 1: Filter by single tag "critical" - should return prod and dev runs
	listResp := server.Client().Get("/api/v1/dag-runs?tags=critical").
		ExpectStatus(http.StatusOK).Send(t)
	var listBody api.ListDAGRuns200JSONResponse
	listResp.Unmarshal(t, &listBody)

	criticalNames := make(map[string]bool)
	for _, run := range listBody.DagRuns {
		criticalNames[run.Name] = true
	}
	require.True(t, criticalNames["tag_filter_prod"], "prod DAG should be in critical filter results")
	require.True(t, criticalNames["tag_filter_dev"], "dev DAG should be in critical filter results")
	require.False(t, criticalNames["tag_filter_test"], "test DAG should NOT be in critical filter results")

	// Test 2: Filter by multiple tags "prod,critical" (AND logic) - should return only prod run
	listResp = server.Client().Get("/api/v1/dag-runs?tags=prod,critical").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	prodCriticalNames := make(map[string]bool)
	for _, run := range listBody.DagRuns {
		prodCriticalNames[run.Name] = true
	}
	require.True(t, prodCriticalNames["tag_filter_prod"], "prod DAG should be in prod+critical filter results")
	require.False(t, prodCriticalNames["tag_filter_dev"], "dev DAG should NOT be in prod+critical filter results")
	require.False(t, prodCriticalNames["tag_filter_test"], "test DAG should NOT be in prod+critical filter results")

	// Test 3: Filter by non-existent tag - should return empty
	listResp = server.Client().Get("/api/v1/dag-runs?tags=nonexistent").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	for _, run := range listBody.DagRuns {
		require.NotContains(t, []string{"tag_filter_prod", "tag_filter_dev", "tag_filter_test"}, run.Name,
			"non-existent tag filter should not return any of our test DAGs")
	}

	// Test 4: Filter by single tag "test" - should return only test run
	listResp = server.Client().Get("/api/v1/dag-runs?tags=test").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	testNames := make(map[string]bool)
	for _, run := range listBody.DagRuns {
		testNames[run.Name] = true
	}
	require.True(t, testNames["tag_filter_test"], "test DAG should be in test filter results")
	require.False(t, testNames["tag_filter_prod"], "prod DAG should NOT be in test filter results")
	require.False(t, testNames["tag_filter_dev"], "dev DAG should NOT be in test filter results")

	// Test 5: Case-insensitive tag matching
	listResp = server.Client().Get("/api/v1/dag-runs?tags=CRITICAL").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	caseInsensitiveNames := make(map[string]bool)
	for _, run := range listBody.DagRuns {
		caseInsensitiveNames[run.Name] = true
	}
	require.True(t, caseInsensitiveNames["tag_filter_prod"], "case-insensitive: prod DAG should be in CRITICAL filter results")
	require.True(t, caseInsensitiveNames["tag_filter_dev"], "case-insensitive: dev DAG should be in CRITICAL filter results")
}

func TestListDAGRunsFilterByPartialName(t *testing.T) {
	server := test.SetupServer(t)

	spec := `steps:
  - name: main
    command: "echo search"`

	for _, dagName := range []string{
		"test-params-flag",
		"other-dag",
		"alpha-test-case",
	} {
		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		resp := server.Client().Post(
			fmt.Sprintf("/api/v1/dags/%s/start", dagName),
			api.ExecuteDAGJSONRequestBody{},
		).ExpectStatus(http.StatusOK).Send(t)

		var body api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &body)
		require.NotEmpty(t, body.DagRunId)
	}

	resp := server.Client().Get("/api/v1/dag-runs?name=test").
		ExpectStatus(http.StatusOK).Send(t)

	var body api.ListDAGRuns200JSONResponse
	resp.Unmarshal(t, &body)

	names := make(map[string]bool)
	for _, run := range body.DagRuns {
		names[run.Name] = true
	}

	require.True(t, names["test-params-flag"])
	require.True(t, names["alpha-test-case"])
	require.False(t, names["other-dag"])
}

func TestListDAGRunsByNameRemainsExact(t *testing.T) {
	server := test.SetupServer(t)

	spec := `steps:
  - name: main
    command: "echo search"`

	for _, dagName := range []string{
		"test-params-flag",
		"alpha-test-case",
	} {
		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		resp := server.Client().Post(
			fmt.Sprintf("/api/v1/dags/%s/start", dagName),
			api.ExecuteDAGJSONRequestBody{},
		).ExpectStatus(http.StatusOK).Send(t)

		var body api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &body)
		require.NotEmpty(t, body.DagRunId)
	}

	resp := server.Client().Get("/api/v1/dag-runs/test-params-flag").
		ExpectStatus(http.StatusOK).Send(t)

	var body api.ListDAGRunsByName200JSONResponse
	resp.Unmarshal(t, &body)

	require.NotEmpty(t, body.DagRuns)
	for _, run := range body.DagRuns {
		require.Equal(t, "test-params-flag", run.Name)
	}
}
