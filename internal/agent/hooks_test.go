package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHooks_AfterToolExec(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()

	var captured struct {
		info   ToolExecInfo
		result ToolOut
	}

	hooks.OnAfterToolExec(func(_ context.Context, info ToolExecInfo, result ToolOut) {
		captured.info = info
		captured.result = result
	})

	info := ToolExecInfo{
		ToolName:  "bash",
		Input:     json.RawMessage(`{"command":"ls"}`),
		SessionID: "sess-1",
		UserID:    "user-1",
		Username:  "alice",
		IPAddress: "10.0.0.1",
		Role:      auth.RoleDeveloper,
	}
	result := ToolOut{Content: "file.txt", IsError: false}

	hooks.RunAfterToolExec(context.Background(), info, result)

	assert.Equal(t, "bash", captured.info.ToolName)
	assert.Equal(t, "sess-1", captured.info.SessionID)
	assert.Equal(t, "user-1", captured.info.UserID)
	assert.Equal(t, "alice", captured.info.Username)
	assert.Equal(t, "10.0.0.1", captured.info.IPAddress)
	assert.Equal(t, auth.RoleDeveloper, captured.info.Role)
	assert.Equal(t, "file.txt", captured.result.Content)
	assert.False(t, captured.result.IsError)
}

func TestHooks_BeforeToolExec_Blocks(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()

	hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
		return errors.New("denied")
	})

	err := hooks.RunBeforeToolExec(context.Background(), ToolExecInfo{ToolName: "bash"})

	require.Error(t, err)
	assert.Equal(t, "denied", err.Error())
}

func TestHooks_BeforeToolExec_Allows(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()

	hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
		return nil
	})

	err := hooks.RunBeforeToolExec(context.Background(), ToolExecInfo{ToolName: "bash"})

	assert.NoError(t, err)
}

func TestHooks_NilHooks(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()

	// No hooks registered, should not panic
	err := hooks.RunBeforeToolExec(context.Background(), ToolExecInfo{})
	assert.NoError(t, err)

	hooks.RunAfterToolExec(context.Background(), ToolExecInfo{}, ToolOut{})
}

func TestHooks_MultipleHooks(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()

	var mu sync.Mutex
	var order []int

	hooks.OnAfterToolExec(func(_ context.Context, _ ToolExecInfo, _ ToolOut) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	hooks.OnAfterToolExec(func(_ context.Context, _ ToolExecInfo, _ ToolOut) {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})
	hooks.OnAfterToolExec(func(_ context.Context, _ ToolExecInfo, _ ToolOut) {
		mu.Lock()
		order = append(order, 3)
		mu.Unlock()
	})

	hooks.RunAfterToolExec(context.Background(), ToolExecInfo{}, ToolOut{})

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestHooks_BeforeToolExec_StopsAtFirstError(t *testing.T) {
	t.Parallel()

	hooks := NewHooks()
	var called []int

	hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
		called = append(called, 1)
		return nil
	})
	hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
		called = append(called, 2)
		return errors.New("blocked")
	})
	hooks.OnBeforeToolExec(func(_ context.Context, _ ToolExecInfo) error {
		called = append(called, 3) // should not be reached
		return nil
	})

	err := hooks.RunBeforeToolExec(context.Background(), ToolExecInfo{})

	require.Error(t, err)
	assert.Equal(t, "blocked", err.Error())
	assert.Equal(t, []int{1, 2}, called)
}
