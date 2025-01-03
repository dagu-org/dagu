package scheduler_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type nodeHelper struct {
	*scheduler.Node
	test.Helper
	reqID string
}

type nodeOption func(*scheduler.NodeData)

func withNodeCmdArgs(cmd string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.CmdWithArgs = cmd
	}
}

func withNodeCommand(command string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Command = command
	}
}

func withNodeSignalOnStop(signal string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.SignalOnStop = signal
	}
}

func withNodeStdout(stdout string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Stdout = stdout
	}
}

func withNodeStderr(stderr string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Stderr = stderr
	}
}

func withNodeScript(script string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Script = script
	}
}

func withNodeOutput(output string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Output = output
	}
}

func setupNode(t *testing.T, opts ...nodeOption) nodeHelper {
	th := test.Setup(t)

	data := scheduler.NodeData{Step: digraph.Step{}}
	for _, opt := range opts {
		opt(&data)
	}

	node := scheduler.NodeWithData(data)
	reqID := uuid.Must(uuid.NewRandom()).String()

	return nodeHelper{node, th, reqID}
}

func (n nodeHelper) Execute(t *testing.T) {
	err := n.Node.Setup(n.Context, n.Config.Paths.LogDir, n.reqID)
	require.NoError(t, err, "failed to setup node")

	err = n.Node.Execute(n.execContext())
	require.NoError(t, err, "failed to execute node")

	err = n.Teardown()
	require.NoError(t, err, "failed to teardown node")
}

func (n nodeHelper) ExecuteFail(t *testing.T, expectedErr string) {
	err := n.Node.Execute(n.execContext())
	require.Error(t, err, "expected error")
	require.Contains(t, err.Error(), expectedErr, "unexpected error")
}

func (n nodeHelper) AssertLogContains(t *testing.T, expected string) {
	dat, err := os.ReadFile(n.Node.LogFilename())
	require.NoErrorf(t, err, "failed to read log file %q", n.Node.LogFilename())
	require.Contains(t, string(dat), expected, "log file does not contain expected string")
}

func (n nodeHelper) AssertOutput(t *testing.T, key, value string) {
	require.NotNil(t, n.Node.Data().Step.OutputVariables, "output variables not set")
	data, ok := n.Node.Data().Step.OutputVariables.Load(key)
	require.True(t, ok, "output variable not found")
	require.Equal(t, fmt.Sprintf("%s=%s", key, value), data, "output variable value mismatch")
}

func (n nodeHelper) execContext() context.Context {
	return digraph.NewContext(n.Context, &digraph.DAG{}, nil, n.reqID, "logFile")
}

func TestNode(t *testing.T) {
	t.Parallel()

	t.Run("Execute", func(t *testing.T) {
		node := setupNode(t, withNodeCommand("true"))
		node.Execute(t)
	})
	t.Run("Error", func(t *testing.T) {
		node := setupNode(t, withNodeCommand("false"))
		node.ExecuteFail(t, "exit status 1")
	})
	t.Run("Signal", func(t *testing.T) {
		node := setupNode(t, withNodeCommand("sleep 3"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			node.Signal(node.Context, syscall.SIGTERM, false)
		}()

		node.SetStatus(scheduler.NodeStatusRunning)

		node.ExecuteFail(t, "signal: terminated")
		require.Equal(t, scheduler.NodeStatusCancel.String(), node.State().Status.String())
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		node := setupNode(t, withNodeCommand("sleep 3"), withNodeSignalOnStop("SIGINT"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			node.Signal(node.Context, syscall.SIGTERM, true) // allow override signal
		}()

		node.SetStatus(scheduler.NodeStatusRunning)

		node.ExecuteFail(t, "signal: interrupt")
		require.Equal(t, scheduler.NodeStatusCancel.String(), node.State().Status.String())
	})
	t.Run("LogOutput", func(t *testing.T) {
		node := setupNode(t, withNodeCommand("echo hello"))
		node.Execute(t)
		node.AssertLogContains(t, "hello")
	})
	t.Run("Stdout", func(t *testing.T) {
		random := path.Join(os.TempDir(), uuid.Must(uuid.NewRandom()).String())
		defer os.Remove(random)

		node := setupNode(t, withNodeCommand("echo hello"), withNodeStdout(random))
		node.Execute(t)

		file := node.Data().Step.Stdout
		dat, _ := os.ReadFile(file)
		require.Equalf(t, "hello\n", string(dat), "unexpected stdout content: %s", string(dat))
	})
	t.Run("Stderr", func(t *testing.T) {
		random := path.Join(os.TempDir(), uuid.Must(uuid.NewRandom()).String())
		defer os.Remove(random)

		node := setupNode(t,
			withNodeCommand("sh"),
			withNodeStderr(random),
			withNodeScript("echo hello >&2"),
		)
		node.Execute(t)

		file := node.Data().Step.Stderr
		dat, _ := os.ReadFile(file)
		require.Equalf(t, "hello\n", string(dat), "unexpected stderr content: %s", string(dat))
	})
	t.Run("Output", func(t *testing.T) {
		node := setupNode(t, withNodeCmdArgs("echo hello"), withNodeOutput("OUTPUT_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_TEST", "hello")
	})
	t.Run("OutputJSON", func(t *testing.T) {
		node := setupNode(t, withNodeCmdArgs(`echo '{"key": "value"}'`), withNodeOutput("OUTPUT_JSON_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_JSON_TEST", `{"key": "value"}`)
	})
	t.Run("OutputJSONUnescaped", func(t *testing.T) {
		node := setupNode(t, withNodeCmdArgs(`echo {\"key\":\"value\"}`), withNodeOutput("OUTPUT_JSON_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_JSON_TEST", `{"key":"value"}`)
	})
	t.Run("OutputSpecialChar", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			CmdWithArgs string
			Want        string
		}{
			{
				CmdWithArgs: `echo "hello\tworld"`,
				Want:        `hello\tworld`,
			},
			{
				CmdWithArgs: `echo hello"\t"world`,
				Want:        `hello\tworld`,
			},
			{
				CmdWithArgs: `echo hello\tworld`,
				Want:        `hello\tworld`,
			},
			{
				CmdWithArgs: `echo hello\nworld`,
				Want:        `hello\nworld`,
			},
			{
				CmdWithArgs: `echo {\"key\":\"value\"}`,
				Want:        `{"key":"value"}`,
			},
			{
				CmdWithArgs: `echo "{\"key\":\"value\"}"`,
				Want:        `{"key":"value"}`,
			},
		}

		for i, tc := range testCases {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				node := setupNode(t, withNodeCmdArgs(tc.CmdWithArgs), withNodeOutput("OUTPUT_SPECIALCHAR_TEST"))
				node.Execute(t)
				node.AssertOutput(t, "OUTPUT_SPECIALCHAR_TEST", tc.Want)
			})
		}
	})
	t.Run("Script", func(t *testing.T) {
		node := setupNode(t, withNodeScript("echo hello"), withNodeOutput("SCRIPT_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "SCRIPT_TEST", "hello")
		// check script file is removed
		scriptFilePath := node.ScriptFilename()
		require.NotEmpty(t, scriptFilePath)
		require.NoFileExists(t, scriptFilePath, "script file not removed")
	})
}
