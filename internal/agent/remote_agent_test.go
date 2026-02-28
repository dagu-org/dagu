package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoteSessionDetail_JSONDeserialization verifies that the remote session
// detail struct correctly deserializes the camelCase JSON returned by the Dagu API.
func TestRemoteSessionDetail_JSONDeserialization(t *testing.T) {
	t.Parallel()

	// This JSON matches the actual Dagu REST API response format (camelCase).
	rawJSON := `{
		"session": {"id": "sess-123"},
		"sessionState": {
			"sessionId": "sess-123",
			"working": true,
			"hasPendingPrompt": true,
			"totalCost": 0.05
		},
		"messages": [
			{
				"id": "msg-1",
				"type": "user",
				"content": "hello",
				"sessionId": "sess-123",
				"sequenceId": 1,
				"createdAt": "2025-01-01T00:00:00Z"
			},
			{
				"id": "msg-2",
				"type": "user_prompt",
				"sessionId": "sess-123",
				"sequenceId": 2,
				"createdAt": "2025-01-01T00:00:01Z",
				"userPrompt": {
					"promptId": "prompt-abc",
					"question": "Approve this command?",
					"promptType": "command_approval"
				}
			},
			{
				"id": "msg-3",
				"type": "assistant",
				"content": "Task completed successfully.",
				"sessionId": "sess-123",
				"sequenceId": 3,
				"createdAt": "2025-01-01T00:00:02Z"
			}
		]
	}`

	var detail remoteSessionDetail
	err := json.Unmarshal([]byte(rawJSON), &detail)
	require.NoError(t, err)

	// Verify sessionState fields (the P0 bug was here — snake_case tags caused zero values).
	assert.True(t, detail.SessionState.Working)
	assert.True(t, detail.SessionState.HasPendingPrompt)
	assert.Equal(t, "sess-123", detail.SessionState.SessionID)

	// Verify messages are parsed.
	require.Len(t, detail.Messages, 3)

	// Verify user_prompt message with camelCase userPrompt field.
	promptMsg := detail.Messages[1]
	assert.Equal(t, "user_prompt", promptMsg.Type)
	require.NotNil(t, promptMsg.UserPrompt)
	assert.Equal(t, "prompt-abc", promptMsg.UserPrompt.PromptID)
	assert.Equal(t, "Approve this command?", promptMsg.UserPrompt.Question)
	assert.Equal(t, "command_approval", promptMsg.UserPrompt.PromptType)

	// Verify assistant content extraction.
	content := extractLastAssistantContent(&detail)
	assert.Equal(t, "Task completed successfully.", content)

	// Verify prompt ID extraction.
	promptID := extractPromptID(&detail)
	assert.Equal(t, "prompt-abc", promptID)

	// Verify prompt info extraction.
	pType, summary := extractPromptInfo(&detail)
	assert.Equal(t, "command_approval", pType)
	assert.Equal(t, "Approve this command?", summary)
}

// TestRemoteGetSession_CamelCaseRoundTrip verifies that remoteGetSession correctly
// deserializes a real API response from an httptest server.
func TestRemoteGetSession_CamelCaseRoundTrip(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/agent/sessions/test-session")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"session": {"id": "test-session"},
			"sessionState": {
				"sessionId": "test-session",
				"working": false,
				"hasPendingPrompt": false
			},
			"messages": [
				{"id": "m1", "type": "assistant", "content": "Done!", "sessionId": "test-session", "sequenceId": 1, "createdAt": "2025-01-01T00:00:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{
		Name:       "test-node",
		APIBaseURL: srv.URL,
		AuthToken:  "test-token",
	}
	client := srv.Client()

	detail, err := remoteGetSession(context.Background(), client, node, "test-session")
	require.NoError(t, err)

	assert.False(t, detail.SessionState.Working)
	assert.False(t, detail.SessionState.HasPendingPrompt)
	assert.Equal(t, "test-session", detail.SessionState.SessionID)

	require.Len(t, detail.Messages, 1)
	assert.Equal(t, "Done!", detail.Messages[0].Content)
}

// TestRemoteRespondToPrompt_SendsCamelCase verifies that the prompt rejection
// request body uses camelCase field names matching the API spec.
func TestRemoteRespondToPrompt_SendsCamelCase(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{
		Name:       "test-node",
		APIBaseURL: srv.URL,
		AuthToken:  "test-token",
	}
	detail := &remoteSessionDetail{
		Messages: []remoteMessage{
			{Type: "user_prompt", UserPrompt: &remotePromptRef{PromptID: "p-123", Question: "approve?"}},
		},
	}

	err := remoteRespondToPrompt(context.Background(), srv.Client(), node, "sess-1", detail)
	require.NoError(t, err)

	// Verify the request body uses "promptId" (camelCase), not "prompt_id" (snake_case).
	assert.Equal(t, "p-123", receivedBody["promptId"])
	assert.Equal(t, true, receivedBody["cancelled"])
	assert.Nil(t, receivedBody["prompt_id"], "should not send snake_case prompt_id")
}

// TestRemoteCreateSession_SendsCamelCase verifies that session creation
// sends the correct camelCase JSON matching the API spec.
func TestRemoteCreateSession_SendsCamelCase(t *testing.T) {
	t.Parallel()

	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sessionId": "sess-1", "status": "accepted"}`))
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{
		Name:       "test-node",
		APIBaseURL: srv.URL,
		AuthToken:  "test-token",
	}

	err := remoteCreateSession(context.Background(), srv.Client(), node, "sess-1", "do something")
	require.NoError(t, err)

	assert.Equal(t, "sess-1", receivedBody["sessionId"])
	assert.Equal(t, "do something", receivedBody["message"])
	assert.Equal(t, true, receivedBody["safeMode"])
	// Must not send a "model" field — remote uses its own default.
	assert.Nil(t, receivedBody["model"])
}

// TestRemotePollSession_FullRoundTrip verifies the complete polling lifecycle:
// working → working → done → extract result.
func TestRemotePollSession_FullRoundTrip(t *testing.T) {
	t.Parallel()

	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := pollCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if n <= 2 {
			// Still working.
			_, _ = w.Write([]byte(`{
				"session": {"id": "s1"},
				"sessionState": {"sessionId": "s1", "working": true, "hasPendingPrompt": false},
				"messages": []
			}`))
			return
		}
		// Done with result.
		_, _ = w.Write([]byte(`{
			"session": {"id": "s1"},
			"sessionState": {"sessionId": "s1", "working": false, "hasPendingPrompt": false},
			"messages": [{"id": "m1", "type": "assistant", "content": "All done!", "sessionId": "s1", "sequenceId": 1, "createdAt": "2025-01-01T00:00:00Z"}]
		}`))
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{
		Name:       "test-node",
		APIBaseURL: srv.URL,
		AuthToken:  "test-token",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, rejected, err := remotePollSession(ctx, srv.Client(), node, "s1")
	require.NoError(t, err)
	assert.Equal(t, "All done!", result)
	assert.Empty(t, rejected)
	assert.GreaterOrEqual(t, int(pollCount.Load()), 3)
}

// TestRemotePollSession_AutoRejectsPrompt verifies that pending prompts are
// auto-rejected and tracked in the rejection list.
func TestRemotePollSession_AutoRejectsPrompt(t *testing.T) {
	t.Parallel()

	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/respond") {
			// Accept prompt rejection.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "accepted"}`))
			return
		}

		n := pollCount.Add(1)
		if n == 1 {
			// First poll: pending prompt.
			_, _ = w.Write([]byte(`{
				"session": {"id": "s1"},
				"sessionState": {"sessionId": "s1", "working": true, "hasPendingPrompt": true},
				"messages": [
					{"id": "m1", "type": "user_prompt", "sessionId": "s1", "sequenceId": 1, "createdAt": "2025-01-01T00:00:00Z",
					 "userPrompt": {"promptId": "p1", "question": "Run rm -rf /?", "promptType": "command_approval"}}
				]
			}`))
			return
		}
		// Second poll: done.
		_, _ = w.Write([]byte(`{
			"session": {"id": "s1"},
			"sessionState": {"sessionId": "s1", "working": false, "hasPendingPrompt": false},
			"messages": [
				{"id": "m2", "type": "assistant", "content": "Cancelled dangerous command.", "sessionId": "s1", "sequenceId": 2, "createdAt": "2025-01-01T00:00:01Z"}
			]
		}`))
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{
		Name:       "test-node",
		APIBaseURL: srv.URL,
		AuthToken:  "test-token",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, rejected, err := remotePollSession(ctx, srv.Client(), node, "s1")
	require.NoError(t, err)
	assert.Equal(t, "Cancelled dangerous command.", result)
	require.Len(t, rejected, 1)
	assert.Equal(t, "command_approval", rejected[0].PromptType)
	assert.Contains(t, rejected[0].Summary, "rm -rf")
}

// TestRemoteDoRequest_SetsContentTypeForPOST verifies that Content-Type is set
// only when a body is provided.
func TestRemoteDoRequest_SetsContentTypeForPOST(t *testing.T) {
	t.Parallel()

	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{Name: "n", APIBaseURL: srv.URL, AuthToken: "tok"}

	// GET request — no Content-Type.
	resp, err := remoteDoRequest(context.Background(), srv.Client(), node, http.MethodGet, "/test", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Empty(t, headers.Get("Content-Type"))

	// POST with body — Content-Type: application/json.
	resp, err = remoteDoRequest(context.Background(), srv.Client(), node, http.MethodPost, "/test", strings.NewReader("{}"))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
}

// TestRemoteDoRequest_ReturnsErrorOnNon2xx verifies that non-2xx responses
// are returned as errors with the response body excerpt.
func TestRemoteDoRequest_ReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"Session not found"}`))
	}))
	defer srv.Close()

	node := &RemoteNodeInfo{Name: "n", APIBaseURL: srv.URL}

	_, err := remoteDoRequest(context.Background(), srv.Client(), node, http.MethodGet, "/test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Session not found")
}

func TestTruncateResult(t *testing.T) {
	t.Parallel()

	t.Run("short content unchanged", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "hello", truncateResult("hello"))
	})

	t.Run("content at limit unchanged", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", remoteAgentMaxResultLen)
		assert.Equal(t, s, truncateResult(s))
	})

	t.Run("long content truncated with marker", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("x", remoteAgentMaxResultLen+500)
		result := truncateResult(s)
		assert.Contains(t, result, "truncated 500 chars")
		assert.True(t, strings.HasPrefix(result, strings.Repeat("x", remoteAgentHeadLen)))
	})
}

func TestAppendRejectionSummary(t *testing.T) {
	t.Parallel()

	rejected := []rejectedPrompt{
		{PromptType: "command_approval", Summary: "Run rm -rf?"},
		{PromptType: "general", Summary: "Continue?"},
	}
	result := appendRejectionSummary("Base result.", rejected)
	assert.Contains(t, result, "Base result.")
	assert.Contains(t, result, "2 prompt(s) were auto-rejected")
	assert.Contains(t, result, "[command_approval]: Run rm -rf?")
	assert.Contains(t, result, "[general]: Continue?")
}

func TestRemoteURL(t *testing.T) {
	t.Parallel()

	node := &RemoteNodeInfo{APIBaseURL: "https://example.com/"}
	assert.Equal(t, "https://example.com/api/v1/agent/sessions", remoteURL(node, "/api/v1/agent/sessions"))

	node2 := &RemoteNodeInfo{APIBaseURL: "https://example.com"}
	assert.Equal(t, "https://example.com/api/v1/agent/sessions", remoteURL(node2, "/api/v1/agent/sessions"))
}

func TestNewRemoteAgentTool_PopulatesEnum(t *testing.T) {
	t.Parallel()

	resolver := &testRemoteNodeResolver{}
	// The test resolver returns empty, so no enum but tool should still be created.
	tool := NewRemoteAgentTool(resolver)
	require.NotNil(t, tool)
	assert.Equal(t, "remote_agent", tool.Function.Name)
}

func TestNewRemoteAgentTool_NilResolverReturnsNil(t *testing.T) {
	t.Parallel()

	reg := findRegistration("remote_agent")
	require.NotNil(t, reg)

	tool := reg.Factory(ToolConfig{})
	assert.Nil(t, tool, "factory should return nil when RemoteNodeResolver is nil")
}

func TestNewListRemoteNodesTool_NilResolverReturnsNil(t *testing.T) {
	t.Parallel()

	reg := findRegistration("list_remote_nodes")
	require.NotNil(t, reg)

	tool := reg.Factory(ToolConfig{})
	assert.Nil(t, tool, "factory should return nil when RemoteNodeResolver is nil")
}

// findRegistration looks up a tool registration by name.
func findRegistration(name string) *ToolRegistration {
	for _, reg := range RegisteredTools() {
		if reg.Name == name {
			return &reg
		}
	}
	return nil
}
