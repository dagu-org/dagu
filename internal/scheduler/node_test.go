package scheduler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/scheduler"
)

func TestExecute(t *testing.T) {
	n := &scheduler.Node{
		Step: &config.Step{
			Command: "true",
		}}
	require.NoError(t, n.Execute())
	assert.Nil(t, n.Error)
}

func TestError(t *testing.T) {
	n := &scheduler.Node{
		Step: &config.Step{
			Command: "false",
		}}
	err := n.Execute()
	assert.True(t, err != nil)
	assert.Equal(t, n.Error, err)
}
