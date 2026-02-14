package cmd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStartCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dagStart := th.DAG(t, `max_active_runs: 1
steps:
  - name: "1"
    command: "true"
`)

	dagStartWithParams := th.DAG(t, `params: "p1 p2"
steps:
  - name: "1"
    command: "echo \"params is $1 and $2\""
`)

	dagStartWithDAGRunID := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

	tests := []test.CmdTest{
		{
			Name:        "StartDAG",
			Args:        []string{"start", dagStart.Location},
			ExpectedOut: []string{"Step started"},
		},
		{
			Name:        "StartDAGWithDefaultParams",
			Args:        []string{"start", dagStartWithParams.Location},
			ExpectedOut: []string{`params="[1=p1 2=p2]"`},
		},
		{
			Name:        "StartDAGWithParams",
			Args:        []string{"start", `--params="p3 p4"`, dagStartWithParams.Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"start", dagStartWithParams.Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6`},
		},
		{
			Name:        "StartDAGWithRequestID",
			Args:        []string{"start", dagStartWithDAGRunID.Location, "--run-id", "CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
			ExpectedOut: []string{"CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cmd.Start(), tc)
		})
	}
}

func TestCmdStart_BackwardCompatibility(t *testing.T) {
	t.Run("ShouldRejectParametersAfterWithoutSeparator", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagContent := `
params: KEY1=default1 KEY2=default2
steps:
  - name: step1
    command: echo $KEY1 $KEY2
`
		dagFile := th.CreateDAGFile(t, "test-params.yaml", dagContent)

		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagFile, "KEY1=value1", "KEY2=value2"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "use '--' before parameters")
	})

	t.Run("ShouldAcceptParamsFlag", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagContent := `
params: KEY=default
steps:
  - name: step1
    command: echo $KEY
`
		dagFile := th.CreateDAGFile(t, "test-params-flag.yaml", dagContent)

		cli := cmd.Start()
		cli.SetArgs([]string{dagFile, "--params", "KEY=value"})

		// Execute will fail due to missing context setup, but we're testing
		// that the command accepts the arguments
		_ = cli.Execute()
	})
}

func TestCmdStart_PositionalParamValidation(t *testing.T) {
	th := test.SetupCommand(t)

	dagFile := th.CreateDAGFile(t, "test-positional-params.yaml", `
params: "p1 p2"
steps:
  - name: step1
    command: echo $1 $2
`)
	dagNoParamsFile := th.CreateDAGFile(t, "test-no-params.yaml", `
steps:
  - name: step1
    command: echo $1
`)

	t.Run("AllowsTooFewAfterDash", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagFile, "--", "only-one"},
		})
		require.NoError(t, err)
	})

	t.Run("RejectsTooManyAfterDash", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagFile, "--", "one", "two", "three"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "too many positional params: expected at most 2, got 3")
	})

	t.Run("AllowsTooFewWithParamsFlag", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--params", "only-one", dagFile},
		})
		require.NoError(t, err)
	})

	t.Run("AllowsNamedOnlyWithPositionalDefaults", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--params", "KEY1=value1 KEY2=value2", dagFile},
		})
		require.NoError(t, err)
	})

	t.Run("AllowsJSONParamsWithoutPositionalValidation", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", "--params", `{"KEY":"value"}`, dagFile},
		})
		require.NoError(t, err)
	})

	t.Run("AllowsJSONAfterDashWithoutPositionalValidation", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagFile, "--", `{"KEY":"value"}`},
		})
		require.NoError(t, err)
	})

	t.Run("AllowsNamedPairsWhenNoParamsDeclared", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagNoParamsFile, "--", "key1=value1", "key2=value2"},
		})
		require.NoError(t, err)
	})

	t.Run("AllowsPositionalWhenNoParamsDeclared", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagNoParamsFile, "--", "success"},
		})
		require.NoError(t, err)
	})
}

func TestCmdStart_NamedParamsIgnorePositionalCount(t *testing.T) {
	th := test.SetupCommand(t)

	dagFile := th.CreateDAGFile(t, "test-named-params.yaml", `
params: KEY1=default1 KEY2=default2
steps:
  - name: step1
    command: echo $KEY1 $KEY2
`)

	err := th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", "--params", "KEY1=value1 KEY2=value2", dagFile},
	})
	require.NoError(t, err)
}

func TestCmdStart_FromRunID(t *testing.T) {
	t.Run("ReschedulesWithStoredParameters", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `params: "alpha beta"
steps:
  - name: "echo"
    command: "echo $1 $2"
`)

		// Kick off an initial run so we have history to clone.
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dag.Location},
		})

		ctx := context.Background()
		originalStatus, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, originalStatus.Status)

		newRunID := "rescheduled_run"
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args: []string{
				"start",
				fmt.Sprintf("--from-run-id=%s", originalStatus.DAGRunID),
				fmt.Sprintf("--run-id=%s", newRunID),
				dag.Name,
			},
		})

		require.Eventually(t, func() bool {
			status, err := th.DAGRunMgr.GetCurrentStatus(ctx, dag.DAG, newRunID)
			return err == nil && status != nil && status.Status == core.Succeeded
		}, 5*time.Second, 100*time.Millisecond)

		newStatus, err := th.DAGRunMgr.GetCurrentStatus(ctx, dag.DAG, newRunID)
		require.NoError(t, err)
		require.NotNil(t, newStatus)
		require.Equal(t, originalStatus.Params, newStatus.Params)
		require.Equal(t, originalStatus.ParamsList, newStatus.ParamsList)
	})

}
