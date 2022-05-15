package scheduler

import (
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
