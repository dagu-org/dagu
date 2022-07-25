package scheduler

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
)

func TestExecute(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "true",
		}}
	require.NoError(t, n.Execute())
	require.Nil(t, n.Error)
}

func TestError(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "false",
		}}
	err := n.Execute()
	require.True(t, err != nil)
	require.Equal(t, n.Error, err)
}

func TestSignal(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "sleep",
			Args:    []string{"100"},
		}}

	go func() {
		time.Sleep(100 * time.Millisecond)
		n.signal(syscall.SIGTERM)
	}()

	n.updateStatus(NodeStatus_Running)
	err := n.Execute()

	require.Error(t, err)
	require.Equal(t, n.Status, NodeStatus_Cancel)
}

func TestLogAndStdout(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "echo",
			Args:    []string{"done"},
			Dir:     os.Getenv("HOME"),
			Stdout:  "stdout.log",
		},
	}

	runTestNode(t, n)

	f := path.Join(os.Getenv("HOME"), n.Step.Stdout)
	dat, _ := os.ReadFile(f)
	require.Equal(t, "done\n", string(dat))

	dat, _ = os.ReadFile(n.logFile.Name())
	require.Equal(t, "done\n", string(dat))
}

func TestNode(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "echo",
			Args:    []string{"hello"},
		},
	}
	n.incDoneCount()
	require.Equal(t, 1, n.ReadDoneCount())

	n.incRetryCount()
	require.Equal(t, 1, n.ReadRetryCount())

	n.id = 1
	n.init()
	require.Nil(t, n.Variables)

	n.id = 0
	n.init()
	require.Equal(t, n.Variables, []string{})
}

func TestOutput(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			CmdWithArgs: "echo hello",
			Output:      "OUTPUT_TEST",
		},
	}
	err := n.setup(os.Getenv("HOME"), "test-request-id-output")
	require.NoError(t, err)
	defer func() {
		_ = n.teardown()
	}()

	runTestNode(t, n)

	dat, _ := os.ReadFile(n.logFile.Name())
	require.Equal(t, "hello\n", string(dat))
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST"))

	// Use the previous output in the subsequent step
	n2 := &Node{
		Step: &config.Step{
			CmdWithArgs: "echo $OUTPUT_TEST",
			Output:      "OUTPUT_TEST2",
		},
	}

	runTestNode(t, n2)
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST2"))

	// Use the previous output in the subsequent step inside a script
	n3 := &Node{
		Step: &config.Step{
			Command: "sh",
			Script:  "echo $OUTPUT_TEST2",
			Output:  "OUTPUT_TEST3",
		},
	}

	runTestNode(t, n3)
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST3"))
}

func TestRunScript(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "sh",
			Args:    []string{},
			Script: `
			  echo hello
			`,
			Output: "SCRIPT_TEST",
		},
	}

	err := n.setup(os.Getenv("HOME"),
		fmt.Sprintf("test-request-id-%d", rand.Int()))
	require.NoError(t, err)

	require.FileExists(t, n.logFile.Name())
	b, _ := os.ReadFile(n.scriptFile.Name())
	require.Equal(t, n.Script, string(b))

	require.NoError(t, err)
	err = n.Execute()
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)

	require.Equal(t, "hello", os.Getenv("SCRIPT_TEST"))
	require.NoFileExists(t, n.scriptFile.Name())
}

func TestTeardown(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: testCommand,
			Args:    []string{},
		},
	}

	runTestNode(t, n)

	// no error since done flag is true
	err := n.teardown()
	require.NoError(t, err)
	require.NoError(t, n.Error)

	// error
	n.done = false
	err = n.teardown()
	require.Error(t, err)
	require.Error(t, n.Error)
}

func runTestNode(t *testing.T, n *Node) {
	t.Helper()
	err := n.setup(os.Getenv("HOME"),
		fmt.Sprintf("test-request-id-%d", rand.Int()))
	require.NoError(t, err)
	err = n.Execute()
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)
}
