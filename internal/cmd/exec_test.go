package cmd_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestExecCommand_Basic(t *testing.T) {
	origShell, shellSet := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if shellSet {
			_ = os.Setenv("SHELL", origShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	th := test.SetupCommand(t)

	testCase := test.CmdTest{
		Name:        "ExecBasicCommand",
		Args:        []string{"exec", "--", "sh", "-c", "echo exec-basic"},
		ExpectedOut: []string{"Executing inline dag-run"},
	}

	th.RunCommand(t, cmd.Exec(), testCase)
}

func TestExecCommand_MissingCommand(t *testing.T) {
	origShell, shellSet := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if shellSet {
			_ = os.Setenv("SHELL", origShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	th := test.SetupCommand(t)

	err := th.RunCommandWithError(t, cmd.Exec(), test.CmdTest{
		Name: "ExecMissingCommand",
		Args: []string{"exec"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "command is required")
}

func TestExecCommand_BaseFileMustExist(t *testing.T) {
	origShell, shellSet := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if shellSet {
			_ = os.Setenv("SHELL", origShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	th := test.SetupCommand(t)

	err := th.RunCommandWithError(t, cmd.Exec(), test.CmdTest{
		Name: "ExecMissingBase",
		Args: []string{"exec", "--base", "missing-base.yaml", "--", "sh", "-c", "echo hi"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "base DAG file")
}

func TestExecCommand_DotenvMustExist(t *testing.T) {
	origShell, shellSet := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if shellSet {
			_ = os.Setenv("SHELL", origShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	th := test.SetupCommand(t)

	err := th.RunCommandWithError(t, cmd.Exec(), test.CmdTest{
		Name: "ExecMissingDotenv",
		Args: []string{"exec", "--dotenv", "missing.env", "--", "sh", "-c", "echo hi"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "dotenv file")
}

func TestExecCommand_WorkerLabelRequiresQueue(t *testing.T) {
	origShell, shellSet := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if shellSet {
			_ = os.Setenv("SHELL", origShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
	})

	th := test.SetupCommand(t)

	err := th.RunCommandWithError(t, cmd.Exec(), test.CmdTest{
		Name: "ExecWorkerLabelWithoutQueue",
		Args: []string{"exec", "--worker-label", "role=batch", "--", "sh", "-c", "echo hi"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "worker selector requires queues")
}
