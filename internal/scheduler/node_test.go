package scheduler

import (
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
)

func TestExecute(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "true",
		}}
	require.NoError(t, n.Execute())
	assert.Nil(t, n.Error)
}

func TestError(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "false",
		}}
	err := n.Execute()
	assert.True(t, err != nil)
	assert.Equal(t, n.Error, err)
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
	err := n.setup(os.Getenv("HOME"), "test-request-id")
	require.NoError(t, err)
	defer func() {
		_ = n.teardown()
	}()

	err = n.Execute()
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)

	f := path.Join(os.Getenv("HOME"), n.Step.Stdout)
	dat, _ := os.ReadFile(f)
	require.Equal(t, "done\n", string(dat))

	dat, _ = os.ReadFile(n.logFile.Name())
	require.Equal(t, "done\n", string(dat))
}

func TestOutput(t *testing.T) {
	n := &Node{
		Step: &config.Step{
			Command: "echo",
			Args:    []string{"hello"},
			Dir:     os.Getenv("HOME"),
			Output:  "OUTPUT_TEST",
		},
	}
	err := n.setup(os.Getenv("HOME"), "test-request-id-output")
	require.NoError(t, err)
	defer func() {
		_ = n.teardown()
	}()

	err = n.Execute()
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)

	dat, _ := os.ReadFile(n.logFile.Name())
	require.Equal(t, "hello\n", string(dat))

	val := os.Getenv("OUTPUT_TEST")
	require.Equal(t, "hello", val)
}
