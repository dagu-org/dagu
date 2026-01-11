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
