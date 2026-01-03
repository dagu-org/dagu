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
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)
}

func TestApproveDAGRunStep(t *testing.T) {
	t.Run("approve waiting step successfully", func(t *testing.T) {
		server := test.SetupServer(t)

		dagSpec := `steps:
  - name: wait-step
    executor:
      type: wait
      config:
        prompt: "Waiting for approval"
  - name: after-wait
    depends: [wait-step]
    command: "echo approved"`

		// Create DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "approve_test_dag",
			Spec: &dagSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start DAG
		startResp := server.Client().Post("/api/v2/dags/approve_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var startBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &startBody)
		require.NotEmpty(t, startBody.DagRunId)

		// Wait for DAG to enter Wait status
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_test_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Wait)
		}, 10*time.Second, 200*time.Millisecond)

		// Approve the waiting step
		approveResp := server.Client().Post(
			fmt.Sprintf("/api/v2/dag-runs/approve_test_dag/%s/steps/wait-step/approve", startBody.DagRunId),
			api.ApproveDAGRunStepJSONRequestBody{},
		).ExpectStatus(http.StatusOK).Send(t)

		var approveBody api.ApproveDAGRunStep200JSONResponse
		approveResp.Unmarshal(t, &approveBody)
		require.Equal(t, startBody.DagRunId, approveBody.DagRunId)
		require.Equal(t, "wait-step", approveBody.StepName)
		require.True(t, approveBody.Resumed)

		// Wait for DAG to complete
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_test_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 200*time.Millisecond)
	})

	t.Run("approve with required inputs", func(t *testing.T) {
		server := test.SetupServer(t)

		dagSpec := `steps:
  - name: wait-with-inputs
    executor:
      type: wait
      config:
        prompt: "Please provide reason"
        input: [reason, approver]
        required: [reason]`

		// Create DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "approve_inputs_dag",
			Spec: &dagSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start DAG
		startResp := server.Client().Post("/api/v2/dags/approve_inputs_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var startBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &startBody)

		// Wait for Wait status
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_inputs_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Wait)
		}, 10*time.Second, 200*time.Millisecond)

		// Approve with inputs
		inputs := map[string]string{"reason": "testing", "approver": "admin"}
		approveResp := server.Client().Post(
			fmt.Sprintf("/api/v2/dag-runs/approve_inputs_dag/%s/steps/wait-with-inputs/approve", startBody.DagRunId),
			api.ApproveDAGRunStepJSONRequestBody{Inputs: &inputs},
		).ExpectStatus(http.StatusOK).Send(t)

		var approveBody api.ApproveDAGRunStep200JSONResponse
		approveResp.Unmarshal(t, &approveBody)
		require.Equal(t, "wait-with-inputs", approveBody.StepName)
		require.True(t, approveBody.Resumed)
	})

	t.Run("missing required inputs", func(t *testing.T) {
		server := test.SetupServer(t)

		dagSpec := `steps:
  - name: wait-required
    executor:
      type: wait
      config:
        input: [reason]
        required: [reason]`

		// Create DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "approve_missing_dag",
			Spec: &dagSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start DAG
		startResp := server.Client().Post("/api/v2/dags/approve_missing_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var startBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &startBody)

		// Wait for Wait status
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_missing_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Wait)
		}, 10*time.Second, 200*time.Millisecond)

		// Approve without required inputs - should fail
		_ = server.Client().Post(
			fmt.Sprintf("/api/v2/dag-runs/approve_missing_dag/%s/steps/wait-required/approve", startBody.DagRunId),
			api.ApproveDAGRunStepJSONRequestBody{},
		).ExpectStatus(http.StatusBadRequest).Send(t)
	})

	t.Run("step not waiting", func(t *testing.T) {
		server := test.SetupServer(t)

		dagSpec := `steps:
  - name: simple-step
    command: "echo done"`

		// Create DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "approve_not_waiting_dag",
			Spec: &dagSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start DAG
		startResp := server.Client().Post("/api/v2/dags/approve_not_waiting_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var startBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &startBody)

		// Wait for DAG to complete
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_not_waiting_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 200*time.Millisecond)

		// Try to approve a step that's already succeeded - should fail
		_ = server.Client().Post(
			fmt.Sprintf("/api/v2/dag-runs/approve_not_waiting_dag/%s/steps/simple-step/approve", startBody.DagRunId),
			api.ApproveDAGRunStepJSONRequestBody{},
		).ExpectStatus(http.StatusBadRequest).Send(t)
	})

	t.Run("step not found", func(t *testing.T) {
		server := test.SetupServer(t)

		dagSpec := `steps:
  - name: wait-step
    executor:
      type: wait`

		// Create DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "approve_not_found_dag",
			Spec: &dagSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start DAG
		startResp := server.Client().Post("/api/v2/dags/approve_not_found_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var startBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &startBody)

		// Wait for Wait status
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/approve_not_found_dag/dag-runs/%s", startBody.DagRunId)
			statusResp := server.Client().Get(url).Send(t)
			if statusResp.Response.StatusCode() != http.StatusOK {
				return false
			}
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Wait)
		}, 10*time.Second, 200*time.Millisecond)

		// Try to approve a non-existent step - should fail with 404
		_ = server.Client().Post(
			fmt.Sprintf("/api/v2/dag-runs/approve_not_found_dag/%s/steps/non-existent-step/approve", startBody.DagRunId),
			api.ApproveDAGRunStepJSONRequestBody{},
		).ExpectStatus(http.StatusNotFound).Send(t)
	})
}

func TestApprovalInputPassing(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: wait-for-approval
    executor:
      type: wait
      config:
        input: [reason, approver]
        required: [reason]
  - name: use-approval-data
    depends: [wait-for-approval]
    command: "echo Approved by ${approver} for reason ${reason}"
    output: RESULT`

	// Create DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "input_passing_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start DAG
	startResp := server.Client().Post("/api/v2/dags/input_passing_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for DAG to enter Wait status
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/input_passing_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}
		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Wait)
	}, 10*time.Second, 200*time.Millisecond)

	// Approve with inputs
	inputs := map[string]string{"reason": "testing", "approver": "admin"}
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/input_passing_dag/%s/steps/wait-for-approval/approve", startBody.DagRunId),
		api.ApproveDAGRunStepJSONRequestBody{Inputs: &inputs},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.True(t, approveBody.Resumed)

	// Wait for DAG to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/input_passing_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}
		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	// Verify that the step output contains the approval inputs
	// Fetch the DAG run details to check step outputs
	url := fmt.Sprintf("/api/v2/dags/input_passing_dag/dag-runs/%s", startBody.DagRunId)
	detailsResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

	var details api.GetDAGDAGRunDetails200JSONResponse
	detailsResp.Unmarshal(t, &details)

	// Find the use-approval-data step and verify it received the inputs
	for _, node := range details.DagRun.Nodes {
		if node.Name == "use-approval-data" {
			require.Equal(t, api.NodeStatus(core.NodeSucceeded), node.Status)
		}
	}
}

func TestApproveSubDAGRunStep(t *testing.T) {
	server := test.SetupServer(t)

	// Create a child DAG with a wait step
	childSpec := `steps:
  - name: child-wait
    executor:
      type: wait
      config:
        prompt: "Approve child"
  - name: child-after
    depends: [child-wait]
    command: "echo child approved"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "subdag_wait_child",
		Spec: &childSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Create parent DAG that calls the child
	parentSpec := `steps:
  - name: call-child
    call: subdag_wait_child
  - name: parent-after
    depends: [call-child]
    command: "echo parent done"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "subdag_wait_parent",
		Spec: &parentSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Start the parent DAG
	startResp := server.Client().Post("/api/v2/dags/subdag_wait_parent/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	// Wait for parent DAG to enter Wait status (child is waiting)
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/subdag_wait_parent/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}
		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Wait)
	}, 15*time.Second, 200*time.Millisecond)

	// Get parent DAG run details to find the sub-DAG run ID
	detailsResp := server.Client().Get(
		fmt.Sprintf("/api/v2/dags/subdag_wait_parent/dag-runs/%s", startBody.DagRunId),
	).ExpectStatus(http.StatusOK).Send(t)

	var details api.GetDAGDAGRunDetails200JSONResponse
	detailsResp.Unmarshal(t, &details)

	// Find the call-child step and get its sub-dag run ID
	var subDAGRunId string
	for _, node := range details.DagRun.Nodes {
		if node.Name == "call-child" && node.SubRuns != nil && len(*node.SubRuns) > 0 {
			subDAGRunId = (*node.SubRuns)[0].DagRunId
			break
		}
	}
	require.NotEmpty(t, subDAGRunId, "sub-DAG run ID should be found")

	// Approve the sub-DAG's wait step
	approveResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/subdag_wait_parent/%s/sub-dag-runs/%s/steps/child-wait/approve",
			startBody.DagRunId, subDAGRunId),
		api.ApproveSubDAGRunStepJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var approveBody api.ApproveSubDAGRunStep200JSONResponse
	approveResp.Unmarshal(t, &approveBody)
	require.Equal(t, subDAGRunId, approveBody.DagRunId)
	require.Equal(t, "child-wait", approveBody.StepName)
	require.True(t, approveBody.Resumed)

	// Wait for parent DAG to complete
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/subdag_wait_parent/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).Send(t)
		if statusResp.Response.StatusCode() != http.StatusOK {
			return false
		}
		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 15*time.Second, 200*time.Millisecond)
}
