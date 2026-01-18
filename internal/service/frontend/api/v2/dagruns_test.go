package api_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestGetDAGRunSpec(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo spec_test"`

	// Create a new DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "spec_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start a DAG run
	startResp := server.Client().Post("/api/v2/dags/spec_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for the DAG run to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/spec_test_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	// Fetch the DAG spec for the DAG run
	specResp := server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/spec", "spec_test_dag", startBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var specBody api.GetDAGRunSpec200JSONResponse
	specResp.Unmarshal(t, &specBody)
	require.NotEmpty(t, specBody.Spec)
	require.Contains(t, specBody.Spec, "echo spec_test")

	// Test 404 for non-existent DAG
	_ = server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/spec", "non_existent_dag", startBody.DagRunId),
	).ExpectStatus(http.StatusNotFound).Send(t)
}

func TestGetDAGRunSpecInline(t *testing.T) {
	server := test.SetupServer(t)

	inlineSpec := `steps:
  - name: inline_step
    command: "echo inline_dag_test"`

	name := "inline_spec_dag"

	// Execute an inline DAG run
	execResp := server.Client().Post("/api/v2/dag-runs", api.ExecuteDAGRunFromSpecJSONRequestBody{
		Spec: inlineSpec,
		Name: &name,
	}).ExpectStatus(http.StatusOK).Send(t)

	var execBody api.ExecuteDAGRunFromSpec200JSONResponse
	execResp.Unmarshal(t, &execBody)
	require.NotEmpty(t, execBody.DagRunId)

	// Wait for the DAG run to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dag-runs/%s/%s", name, execBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		if dagRunStatus.DagRunDetails.Status == api.Status(core.Succeeded) {
			return true
		} else if dagRunStatus.DagRunDetails.Status == api.Status(core.Running) {
			return true
		}
		t.Logf("DAG run status: %s", dagRunStatus.DagRunDetails.StatusLabel)
		logData, _ := os.ReadFile(dagRunStatus.DagRunDetails.Log)
		t.Fatalf("DAG run failed: %s", string(logData))
		panic("DAG run failed")
	}, 10*time.Second, 200*time.Millisecond)

	// Fetch the spec for the inline DAG run (should use YamlData from dag.json)
	specResp := server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/spec", name, execBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var specBody api.GetDAGRunSpec200JSONResponse
	specResp.Unmarshal(t, &specBody)
	require.NotEmpty(t, specBody.Spec)
	require.Contains(t, specBody.Spec, "echo inline_dag_test")
}

func TestGetSubDAGRunSpec(t *testing.T) {
	server := test.SetupServer(t)

	// Create a parent DAG with an inline sub-DAG definition
	dagSpec := `steps:
  - name: call_child
    call: child_dag
    params: "MSG=hello"

---

name: child_dag
params:
  - MSG
steps:
  - name: echo_message
    command: "echo ${MSG}_from_child"`

	// Create the parent DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "parent_dag_for_subdag_spec",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the parent DAG
	startResp := server.Client().Post("/api/v2/dags/parent_dag_for_subdag_spec/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for the parent DAG to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/parent_dag_for_subdag_spec/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	// Get the parent DAG run details to extract sub-DAG run ID
	detailsResp := server.Client().Get(
		fmt.Sprintf("/api/v2/dags/parent_dag_for_subdag_spec/dag-runs/%s", startBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var detailsBody api.GetDAGDAGRunDetails200JSONResponse
	detailsResp.Unmarshal(t, &detailsBody)
	require.Len(t, detailsBody.DagRun.Nodes, 1, "Expected 1 node (the call_child step)")

	// Extract the sub-DAG run ID from the call step
	callNode := detailsBody.DagRun.Nodes[0]
	require.Equal(t, "call_child", callNode.Step.Name)
	require.NotNil(t, callNode.SubRuns, "Expected SubRuns to be present")
	require.Len(t, *callNode.SubRuns, 1, "Expected exactly one sub-DAG run")
	subDAGRunID := (*callNode.SubRuns)[0].DagRunId

	// Test 1: Fetch the sub-DAG spec successfully
	subSpecResp := server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, subDAGRunID),
	).ExpectStatus(http.StatusOK).Send(t)

	var subSpecBody api.GetSubDAGRunSpec200JSONResponse
	subSpecResp.Unmarshal(t, &subSpecBody)
	require.NotEmpty(t, subSpecBody.Spec, "Sub-DAG spec should not be empty")
	require.Contains(t, subSpecBody.Spec, "child_dag", "Spec should contain child_dag name")
	require.Contains(t, subSpecBody.Spec, "echo_message", "Spec should contain echo_message step")
	require.Contains(t, subSpecBody.Spec, "echo ${MSG}_from_child", "Spec should contain the command")

	// Test 2: 404 for non-existent sub-DAG run ID
	_ = server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, "non_existent_sub_dag_id"),
	).ExpectStatus(http.StatusNotFound).Send(t)

	// Test 3: 404 for non-existent parent DAG
	_ = server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/non_existent_dag/%s/sub-dag-runs/%s/spec",
			startBody.DagRunId, subDAGRunID),
	).ExpectStatus(http.StatusNotFound).Send(t)

	// Test 4: 404 for non-existent parent DAG run ID
	_ = server.Client().Get(
		fmt.Sprintf("/api/v2/dag-runs/parent_dag_for_subdag_spec/%s/sub-dag-runs/%s/spec",
			"non_existent_run_id", subDAGRunID),
	).ExpectStatus(http.StatusNotFound).Send(t)
}

func TestApproveDAGRunStep(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo approved"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v2/dags/approval_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/approval_test_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Waiting)
	}, 10*time.Second, 100*time.Millisecond)

	// Approve the wait step
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/approval_test_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.Equal(t, startBody.DagRunId, approveBody.DagRunId)
	require.Equal(t, "wait-step", approveBody.StepName)
	require.True(t, approveBody.Resumed)

	// Wait for DAG to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/approval_test_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 100*time.Millisecond)
}

func TestApproveDAGRunStepWithInputs(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please provide reason"
      input:
        - reason
        - approver
      required:
        - reason
  - name: after-wait
    depends: [wait-step]
    command: "echo reason=$reason approver=$approver"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_inputs_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v2/dags/approval_inputs_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/approval_inputs_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Waiting)
	}, 10*time.Second, 100*time.Millisecond)

	// Approve with inputs
	inputs := map[string]string{
		"reason":   "testing",
		"approver": "test-user",
	}
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/approval_inputs_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{Inputs: &inputs},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.True(t, approveBody.Resumed)

	// Wait for DAG to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/approval_inputs_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 100*time.Millisecond)
}

func TestApproveDAGRunStepMissingRequired(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please provide reason"
      input:
        - reason
      required:
        - reason
  - name: after-wait
    depends: [wait-step]
    command: "echo done"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "approval_required_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG
	startResp := server.Client().Post("/api/v2/dags/approval_required_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	// Wait for DAG to enter Wait status
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/approval_required_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Waiting)
	}, 10*time.Second, 100*time.Millisecond)

	// Try to approve without required input - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/approval_required_dag/%s/steps/wait-step/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestApproveDAGRunStepNotWaiting(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo done"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "no_wait_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start and wait for completion
	startResp := server.Client().Post("/api/v2/dags/no_wait_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/no_wait_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 100*time.Millisecond)

	// Try to approve a step that's not waiting - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/no_wait_dag/%s/steps/main/approve", startBody.DagRunId),
		api.ApproveStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestRejectDAGRunStep(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo should not run"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "rejection_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v2/dags/rejection_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/rejection_test_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Waiting)
	}, 10*time.Second, 100*time.Millisecond)

	// Reject the wait step
	reason := "test rejection reason"
	rejectResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/rejection_test_dag/%s/steps/wait-step/reject", startBody.DagRunId),
		api.RejectStepRequest{Reason: &reason},
	).ExpectStatus(http.StatusOK).Send(t)

	var rejectBody api.RejectDAGRunStep200JSONResponse
	rejectResp.Unmarshal(t, &rejectBody)
	require.Equal(t, startBody.DagRunId, rejectBody.DagRunId)
	require.Equal(t, "wait-step", rejectBody.StepName)

	// Verify DAG status is Rejected
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/rejection_test_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Rejected)
	}, 10*time.Second, 100*time.Millisecond)
}

func TestRejectDAGRunStepNotWaiting(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo done"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reject_not_waiting_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v2/dags/reject_not_waiting_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reject_not_waiting_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 100*time.Millisecond)

	// Try to reject a step that's not waiting - should fail
	_ = server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/reject_not_waiting_dag/%s/steps/main/reject", startBody.DagRunId),
		api.RejectStepRequest{},
	).ExpectStatus(http.StatusBadRequest).Send(t)
}

func TestRescheduleDAGRun(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo reschedule"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v2/dags/reschedule_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	rescheduleResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/reschedule", "reschedule_dag", startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var rescheduleBody api.RescheduleDAGRun200JSONResponse
	rescheduleResp.Unmarshal(t, &rescheduleBody)
	require.NotEmpty(t, rescheduleBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_dag/dag-runs/%s", rescheduleBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)
}

func TestExecuteDAGSync(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: echo-step
    command: "echo hello sync"`

	// Create a new DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_test_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously with timeout
	timeout := 30
	syncResp := server.Client().Post("/api/v2/dags/sync_test_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
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

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_timeout_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously with a very short timeout (1 second)
	timeout := 1
	syncResp := server.Client().Post("/api/v2/dags/sync_timeout_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
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

	// Create a DAG with HITL step that will wait for approval
	dagSpec := `steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Approve this"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_waiting_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Execute synchronously - should return when DAG reaches waiting status
	timeout := 30
	syncResp := server.Client().Post("/api/v2/dags/sync_waiting_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
		Timeout: timeout,
	}).ExpectStatus(http.StatusOK).Send(t)

	var syncBody api.ExecuteDAGSync200JSONResponse
	syncResp.Unmarshal(t, &syncBody)

	// Should return with waiting status (not timeout)
	require.NotEmpty(t, syncBody.DagRun.DagRunId)
	require.Equal(t, api.Status(core.Waiting), syncBody.DagRun.Status)
	require.Equal(t, api.StatusLabel("waiting"), syncBody.DagRun.StatusLabel)
}

func TestExecuteDAGSyncSingleton(t *testing.T) {
	server := test.SetupServer(t)

	// Create a DAG with a slow step
	dagSpec := `steps:
  - name: slow-step
    command: "sleep 5"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "sync_singleton_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the DAG asynchronously first
	startResp := server.Client().Post("/api/v2/dags/sync_singleton_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Try to start another sync execution with singleton mode - should fail with 409
	singleton := true
	timeout := 30
	_ = server.Client().Post("/api/v2/dags/sync_singleton_dag/start-sync", api.ExecuteDAGSyncJSONRequestBody{
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
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_prod",
		Spec: &dagSpecProd,
	}).ExpectStatus(http.StatusCreated).Send(t)

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_dev",
		Spec: &dagSpecDev,
	}).ExpectStatus(http.StatusCreated).Send(t)

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "tag_filter_test",
		Spec: &dagSpecTest,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start DAG runs for each
	var prodRunId, devRunId, testRunId string

	startResp := server.Client().Post("/api/v2/dags/tag_filter_prod/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)
	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	prodRunId = startBody.DagRunId

	startResp = server.Client().Post("/api/v2/dags/tag_filter_dev/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)
	startResp.Unmarshal(t, &startBody)
	devRunId = startBody.DagRunId

	startResp = server.Client().Post("/api/v2/dags/tag_filter_test/start", api.ExecuteDAGJSONRequestBody{}).
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
			url := fmt.Sprintf("/api/v2/dags/%s/dag-runs/%s", pair.name, pair.runId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 200*time.Millisecond)
	}

	// Test 1: Filter by single tag "critical" - should return prod and dev runs
	listResp := server.Client().Get("/api/v2/dag-runs?tags=critical").
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
	listResp = server.Client().Get("/api/v2/dag-runs?tags=prod,critical").
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
	listResp = server.Client().Get("/api/v2/dag-runs?tags=nonexistent").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	for _, run := range listBody.DagRuns {
		require.NotContains(t, []string{"tag_filter_prod", "tag_filter_dev", "tag_filter_test"}, run.Name,
			"non-existent tag filter should not return any of our test DAGs")
	}

	// Test 4: Filter by single tag "test" - should return only test run
	listResp = server.Client().Get("/api/v2/dag-runs?tags=test").
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
	listResp = server.Client().Get("/api/v2/dag-runs?tags=CRITICAL").
		ExpectStatus(http.StatusOK).Send(t)
	listResp.Unmarshal(t, &listBody)

	caseInsensitiveNames := make(map[string]bool)
	for _, run := range listBody.DagRuns {
		caseInsensitiveNames[run.Name] = true
	}
	require.True(t, caseInsensitiveNames["tag_filter_prod"], "case-insensitive: prod DAG should be in CRITICAL filter results")
	require.True(t, caseInsensitiveNames["tag_filter_dev"], "case-insensitive: dev DAG should be in CRITICAL filter results")
}
