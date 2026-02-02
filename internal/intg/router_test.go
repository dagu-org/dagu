package intg_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRouterExecutor(t *testing.T) {
	t.Parallel()

	t.Run("ExactMatch", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - INPUT: exact_value
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "exact_value": [route_a]
      "other": [route_b]

  - name: route_a
    command: echo "Route A executed"
    output: RESULT_A

  - name: route_b
    command: echo "Route B executed"
    output: RESULT_B
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// route_a should execute, route_b should be skipped
		dag.AssertOutputs(t, map[string]any{
			"RESULT_A": "Route A executed",
		})
	})

	t.Run("RegexMatch", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - INPUT: apple_pie
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "re:^apple.*": [route_a]
      "re:^banana.*": [route_b]

  - name: route_a
    command: echo "Apple route"
    output: RESULT_A

  - name: route_b
    command: echo "Banana route"
    output: RESULT_B
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT_A": "Apple route",
		})
	})

	t.Run("CatchAllRoute", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - INPUT: unknown_value
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "specific": [route_a]
      "re:.*": [default_route]

  - name: route_a
    command: echo "Specific route"
    output: RESULT_A

  - name: default_route
    command: echo "Default route"
    output: RESULT_DEFAULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// Only default_route should execute
		dag.AssertOutputs(t, map[string]any{
			"RESULT_DEFAULT": "Default route",
		})
	})

	t.Run("MultipleTargetsPerRoute", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
env:
  - INPUT: trigger
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "trigger": [step_a, step_b]

  - name: step_a
    command: echo "Step A"
    output: RESULT_A

  - name: step_b
    command: echo "Step B"
    output: RESULT_B

  - name: step_c
    command: echo "Step C"
    output: RESULT_C
    depends:
      - step_a
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// Both step_a and step_b should execute
		dag.AssertOutputs(t, map[string]any{
			"RESULT_A": "Step A",
			"RESULT_B": "Step B",
			"RESULT_C": "Step C",
		})
	})

	t.Run("MultipleMatchingRoutes", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - INPUT: success_code
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "re:^success.*": [handle_success]
      "re:.*_code$": [handle_code]
      "re:.*": [catch_all]

  - name: handle_success
    command: echo "Success handler"
    output: SUCCESS

  - name: handle_code
    command: echo "Code handler"
    output: CODE

  - name: catch_all
    command: echo "Catch all"
    output: CATCH_ALL
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// All three routes match "success_code"
		dag.AssertOutputs(t, map[string]any{
			"SUCCESS":   "Success handler",
			"CODE":      "Code handler",
			"CATCH_ALL": "Catch all",
		})
	})

	t.Run("NoMatchingRoute", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
env:
  - INPUT: no_match
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "specific_value": [route_a]

  - name: route_a
    command: echo "Route A"
    output: RESULT_A

  - name: always_runs
    command: echo "Always runs"
    output: ALWAYS
    depends:
      - router
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// route_a is skipped, always_runs executes
		dag.AssertOutputs(t, map[string]any{
			"ALWAYS": "Always runs",
		})
	})

	t.Run("RouterWithEnvVarExpansion", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - STATUS: production
steps:
  - name: router
    type: router
    value: ${STATUS}
    routes:
      "production": [prod_handler]
      "staging": [staging_handler]

  - name: prod_handler
    command: echo "Production"
    output: ENV

  - name: staging_handler
    command: echo "Staging"
    output: ENV
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"ENV": "Production",
		})
	})
}

func TestRouterComplexScenarios(t *testing.T) {
	t.Parallel()

	t.Run("ChainedRouters", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
env:
  - CATEGORY: electronics
  - SUBCATEGORY: phone
steps:
  - name: category_router
    type: router
    value: ${CATEGORY}
    routes:
      "electronics": [electronics_router]
      "clothing": [clothing_handler]

  - name: electronics_router
    type: router
    value: ${SUBCATEGORY}
    routes:
      "phone": [phone_handler]
      "laptop": [laptop_handler]

  - name: phone_handler
    command: echo "Phone"
    output: PHONE_RESULT

  - name: laptop_handler
    command: echo "Laptop"
    output: LAPTOP_RESULT

  - name: clothing_handler
    command: echo "Clothing"
    output: CLOTHING_RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"PHONE_RESULT": "Phone",
		})

		// Verify correct routing path: electronics -> phone
		status := agent.Status(agent.Context)
		for _, node := range status.Nodes {
			switch node.Step.Name {
			case "category_router", "electronics_router", "phone_handler":
				require.Equal(t, core.NodeSucceeded, node.Status, "%s should succeed", node.Step.Name)
			case "laptop_handler", "clothing_handler":
				require.Equal(t, core.NodeSkipped, node.Status, "%s should be skipped", node.Step.Name)
			}
		}
	})

	t.Run("BranchesWithMultipleSteps", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
env:
  - MODE: premium
steps:
  - name: router
    type: router
    value: ${MODE}
    routes:
      "premium": [premium_step1]
      "standard": [standard_step1]

  # Premium branch: 3 steps - each step in the chain is a router target
  - name: premium_step1
    command: echo "Premium-1"
    output: P1

  - name: premium_step2
    command: echo "Premium-2"
    output: P2
    depends: [premium_step1]

  - name: premium_step3
    command: echo "Premium-3"
    output: FINAL
    depends: [premium_step2]

  # Standard branch: single step (router target)
  - name: standard_step1
    command: echo "Standard-1"
    output: S1
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		// Premium branch executed
		dag.AssertOutputs(t, map[string]any{
			"P1":    "Premium-1",
			"P2":    "Premium-2",
			"FINAL": "Premium-3",
		})

		// Verify standard_step1 was skipped
		status := agent.Status(agent.Context)
		for _, node := range status.Nodes {
			switch node.Step.Name {
			case "router", "premium_step1", "premium_step2", "premium_step3":
				require.Equal(t, core.NodeSucceeded, node.Status, "%s should succeed", node.Step.Name)
			case "standard_step1":
				require.Equal(t, core.NodeSkipped, node.Status, "%s should be skipped", node.Step.Name)
			}
		}
	})

	t.Run("ComplexDAGTopology", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
env:
  - TRIGGER: full
steps:
  - name: setup
    command: echo "Setup"
    output: SETUP

  - name: router
    type: router
    value: ${TRIGGER}
    routes:
      "full": [full_process_a, full_process_b]
      "minimal": [minimal_process]
    depends: [setup]

  # Full processing path (two parallel steps)
  - name: full_process_a
    command: echo "FullA"
    output: FULL_A

  - name: full_process_b
    command: echo "FullB"
    output: FULL_B

  # Minimal processing path
  - name: minimal_process
    command: echo "Minimal"
    output: MINIMAL
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)

		// Verify routing: full path executed, minimal skipped
		status := agent.Status(agent.Context)
		for _, node := range status.Nodes {
			switch node.Step.Name {
			case "setup", "router", "full_process_a", "full_process_b":
				require.Equal(t, core.NodeSucceeded, node.Status, "%s should succeed", node.Step.Name)
			case "minimal_process":
				require.Equal(t, core.NodeSkipped, node.Status, "%s should be skipped", node.Step.Name)
			}
		}

		// Full branches execute since TRIGGER=full
		dag.AssertOutputs(t, map[string]any{
			"SETUP":  "Setup",
			"FULL_A": "FullA",
			"FULL_B": "FullB",
		})
	})

	t.Run("RouterWithStepOutput", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
steps:
  - name: check_status
    command: echo "success"
    output: STATUS

  - name: router
    type: router
    value: ${STATUS}
    routes:
      "success": [success_handler]
      "failure": [failure_handler]
    depends: [check_status]

  - name: success_handler
    command: echo "Handling success"
    output: SUCCESS_RESULT

  - name: failure_handler
    command: echo "Handling failure"
    output: FAILURE_RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"STATUS":         "success",
			"SUCCESS_RESULT": "Handling success",
		})

		// Verify correct routing based on step output
		status := agent.Status(agent.Context)
		for _, node := range status.Nodes {
			switch node.Step.Name {
			case "check_status", "router", "success_handler":
				require.Equal(t, core.NodeSucceeded, node.Status, "%s should succeed", node.Step.Name)
			case "failure_handler":
				require.Equal(t, core.NodeSkipped, node.Status, "%s should be skipped", node.Step.Name)
			}
		}
	})
}

func TestRouterStepStatus(t *testing.T) {
	t.Parallel()

	t.Run("SkippedStepsHaveCorrectStatus", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
env:
  - INPUT: route_a
steps:
  - name: router
    type: router
    value: ${INPUT}
    routes:
      "route_a": [step_a]
      "route_b": [step_b]

  - name: step_a
    command: echo "A"

  - name: step_b
    command: echo "B"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		// Verify the status
		status := agent.Status(agent.Context)
		require.Equal(t, core.Succeeded, status.Status)

		// Check individual node statuses
		for _, node := range status.Nodes {
			switch node.Step.Name {
			case "router":
				require.Equal(t, core.NodeSucceeded, node.Status, "router should succeed")
			case "step_a":
				require.Equal(t, core.NodeSucceeded, node.Status, "step_a should succeed")
			case "step_b":
				require.Equal(t, core.NodeSkipped, node.Status, "step_b should be skipped")
			}
		}
	})
}

func TestRouterValidation(t *testing.T) {
	t.Parallel()

	t.Run("DuplicateTargetValidation", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Write DAG file manually to test validation error
		dagContent := `
type: graph
env:
  - MODE: full
steps:
  - name: router
    type: router
    value: ${MODE}
    routes:
      "full": [process_a, process_b]
      "minimal": [process_a]

  - name: process_a
    command: echo "A"

  - name: process_b
    command: echo "B"
`
		dagFile := th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "duplicate_target_test.yaml", []byte(dagContent))

		_, err := spec.Load(th.Context, dagFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "targeted by multiple routes")
	})
}
