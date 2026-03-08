package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWaitStepApproval(t *testing.T) {
	t.Run("WaitStepEntersWaitStatus", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: echo "approved"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "wait-step", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status)
		require.Equal(t, "after-wait", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[1].Status)
	})

	t.Run("WaitStepBlocksDependentNodes", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: before-wait
    command: echo "before"
  - name: wait-step
    depends: [before-wait]
    type: hitl
  - name: after-wait-1
    depends: [wait-step]
    command: echo "after1"
  - name: after-wait-2
    depends: [wait-step]
    command: echo "after2"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 4)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status, "before-wait should succeed")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[1].Status, "wait-step should be waiting")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[2].Status, "after-wait-1 should not start")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[3].Status, "after-wait-2 should not start")
	})

	t.Run("LegacyHITLWithApprovalTranslation", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: wait-step
    type: hitl
    config:
      prompt: "Please approve"
      input: [FEEDBACK]
      required: [FEEDBACK]
  - name: after-wait
    depends: [wait-step]
    command: echo "approved"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Verify that the HITL config was translated to approval config
		require.NotNil(t, dagRunStatus.Nodes[0].Step.Approval, "HITL step should have approval config from translation")
		require.Equal(t, "Please approve", dagRunStatus.Nodes[0].Step.Approval.Prompt)
		require.Equal(t, []string{"FEEDBACK"}, dagRunStatus.Nodes[0].Step.Approval.Input)
		require.Equal(t, []string{"FEEDBACK"}, dagRunStatus.Nodes[0].Step.Approval.Required)
	})

	t.Run("ParallelBranchWithWaitStep", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: branch-a-1
    command: echo "a1"
  - name: branch-a-2
    depends: [branch-a-1]
    command: echo "a2"
  - name: wait-branch
    type: hitl
  - name: after-wait
    depends: [wait-branch]
    command: echo "after wait"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 4)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status, "branch-a-1 should succeed")
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status, "branch-a-2 should succeed")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[2].Status, "wait-branch should be waiting")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[3].Status, "after-wait should not start")
	})
}

func TestApprovalField(t *testing.T) {
	t.Run("ApprovalFieldOnCommandStep", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: review-step
    command: echo "ready for review"
    approval:
      prompt: "Please review the output"
  - name: after-review
    depends: [review-step]
    command: echo "approved"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		// The command step should have executed (produced stdout) then entered waiting
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status, "review-step should be waiting")
		require.NotEmpty(t, dagRunStatus.Nodes[0].Stdout, "review-step should have stdout from command execution")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[1].Status, "after-review should not start")
	})

	t.Run("ApprovalFieldOnScriptStep", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: script-step
    script: |
      echo "script output line 1"
      echo "script output line 2"
    approval:
      prompt: "Review the script output"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status, "script-step should be waiting")
		require.NotEmpty(t, dagRunStatus.Nodes[0].Stdout, "script-step should have stdout")
	})

	t.Run("ApprovalFieldWithInputConfig", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: review-step
    command: echo "needs feedback"
    approval:
      prompt: "Please provide feedback"
      input: [FEEDBACK, NOTES]
      required: [FEEDBACK]
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status)
		require.NotNil(t, dagRunStatus.Nodes[0].Step.Approval)
		require.Equal(t, "Please provide feedback", dagRunStatus.Nodes[0].Step.Approval.Prompt)
		require.Equal(t, []string{"FEEDBACK", "NOTES"}, dagRunStatus.Nodes[0].Step.Approval.Input)
		require.Equal(t, []string{"FEEDBACK"}, dagRunStatus.Nodes[0].Step.Approval.Required)
	})

	t.Run("MultipleApprovalSteps", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: step-a
    command: echo "step a output"
    approval:
      prompt: "Approve step A"
  - name: step-b
    command: echo "step b output"
    approval:
      prompt: "Approve step B"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status, "step-a should be waiting")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[1].Status, "step-b should be waiting")
	})

	t.Run("ApprovalStepWithDependency", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: step-a
    command: echo "upstream"
  - name: step-b
    depends: [step-a]
    command: echo "needs approval"
    approval:
      prompt: "Review step-b"
  - name: step-c
    depends: [step-b]
    command: echo "downstream"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 3)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status, "step-a should succeed")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[1].Status, "step-b should be waiting")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[2].Status, "step-c should not start")
	})

	t.Run("ApprovalFieldOnCallStep", func(t *testing.T) {
		th := test.Setup(t)

		// Create the sub-DAG first
		subDAG := th.DAG(t, `
type: graph
steps:
  - name: sub-step
    command: echo "sub-dag output"
`)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: call-step
    call: "`+subDAG.DAG.Name+`"
    approval:
      prompt: "Review sub-DAG results"
  - name: after-call
    depends: [call-step]
    command: echo "after call"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Waiting)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status, "call-step should be waiting after sub-DAG completes")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[1].Status, "after-call should not start")
	})
}
