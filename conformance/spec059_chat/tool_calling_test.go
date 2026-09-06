// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec059_chat_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// with.tools references another DAG as a callable function: the LLM is
// offered its params as a JSON schema, a requested call runs it as a real
// sub-DAG with the LLM's arguments as DAG params, and its declared outputs
// are JSON-encoded back to the LLM as the tool result.
func TestToolCallingBasic(t *testing.T) {
	t.Parallel()

	requests := make(chan toolChatRequest, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req toolChatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		requests <- req

		w.Header().Set("Content-Type", "application/json")
		if len(req.Messages) == 1 {
			// First turn: request the tool call.
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get-weather","arguments":"{\"CITY\":\"Paris\"}"}}]},
				"finish_reason":"tool_calls"}]}`))
			return
		}
		// Second turn: final answer using the tool result.
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"It is 22C in Paris."},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(srv.Close)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv([]string{"LLM_BASE_URL=" + srv.URL}, "start", "tool_calling_basic.yaml")
	srv.Close()
	close(requests)
	result.ExpectExitCode(0)

	data, err := os.ReadFile(dagu.ProjectPath("result.out"))
	require.NoError(t, err)
	require.Equal(t, "It is 22C in Paris.\n", string(data))

	var captured []toolChatRequest
	for req := range requests {
		captured = append(captured, req)
	}
	require.Len(t, captured, 2)

	// First request offers the tool DAG's params as a JSON schema.
	require.Len(t, captured[0].Tools, 1)
	require.Equal(t, "get-weather", captured[0].Tools[0].Function.Name)
	require.Equal(t, []string{"CITY"}, captured[0].Tools[0].Function.Parameters.Required)

	// Second request carries the assistant tool call and the tool's result,
	// which is the sub-DAG's declared output JSON-encoded as the tool content.
	require.Len(t, captured[1].Messages, 3)
	toolMsg := captured[1].Messages[2]
	require.Equal(t, "tool", toolMsg.Role)
	require.Equal(t, "call_1", toolMsg.ToolCallID)
	require.JSONEq(t, `{"WEATHER_JSON":"{\"temp_c\": 22, \"city\": \"Paris\"}"}`, toolMsg.Content)
}

// A tool call naming a DAG outside with.tools is a non-fatal error: its
// result content tells the LLM the tool was not found, and the loop
// continues to a final response rather than failing the run.
func TestToolCallingUnknownToolIsNonFatal(t *testing.T) {
	t.Parallel()

	requestCount := 0
	var secondRequest toolChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"not-a-real-tool","arguments":"{}"}}]},
				"finish_reason":"tool_calls"}]}`))
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&secondRequest))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"tool was unavailable"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(srv.Close)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv([]string{"LLM_BASE_URL=" + srv.URL}, "start", "tool_calling_unknown_tool.yaml")
	result.ExpectExitCode(0)

	data, err := os.ReadFile(dagu.ProjectPath("result.out"))
	require.NoError(t, err)
	require.Equal(t, "tool was unavailable\n", string(data))

	require.Equal(t, 2, requestCount)
	toolMsg := secondRequest.Messages[2]
	require.Equal(t, "tool", toolMsg.Role)
	require.Equal(t, `Error: tool "not-a-real-tool" not found`, toolMsg.Content)
}

// A tool DAG that itself fails is also non-fatal: the failure is reported as
// the tool's result content, and the loop continues.
func TestToolCallingFailedToolDAG(t *testing.T) {
	t.Parallel()

	requestCount := 0
	var secondRequest toolChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"broken-tool","arguments":"{}"}}]},
				"finish_reason":"tool_calls"}]}`))
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&secondRequest))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"the tool failed"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(srv.Close)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv([]string{"LLM_BASE_URL=" + srv.URL}, "start", "tool_calling_failed_tool.yaml")
	result.ExpectExitCode(0)

	data, err := os.ReadFile(dagu.ProjectPath("result.out"))
	require.NoError(t, err)
	require.Equal(t, "the tool failed\n", string(data))

	require.Equal(t, 2, requestCount)
	toolMsg := secondRequest.Messages[2]
	require.Equal(t, "tool", toolMsg.Role)
	require.Equal(t, "Error: execution failed: exit status 1", toolMsg.Content)
}

// max_tool_iterations bounds the tool loop: reaching it still succeeds the
// step (it is not a failure), stopping after exactly that many requests.
// With no override, the default is 10.
func TestToolCallingMaxIterationsReached(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		fixture       string
		maxIterations int
	}{
		{"tool_calling_default_max_iterations.yaml", 10},
		{"tool_calling_custom_max_iterations.yaml", 3},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			requestCount := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				// Always request another tool call; the LLM never finishes on its own.
				_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"still working",
					"tool_calls":[{"id":"call_X","type":"function","function":{"name":"loop-tool","arguments":"{}"}}]},
					"finish_reason":"tool_calls"}]}`))
			}))
			t.Cleanup(srv.Close)

			dagu := harness.NewRunner(t)
			result := dagu.RunWithEnv([]string{"LLM_BASE_URL=" + srv.URL}, "start", tc.fixture)
			result.ExpectExitCode(0)

			require.Equal(t, tc.maxIterations, requestCount)

			data, err := os.ReadFile(dagu.ProjectPath("result.out"))
			require.NoError(t, err)
			require.Equal(t, "still working\n", string(data))
		})
	}
}

type toolChatRequest struct {
	Model      string             `json:"model"`
	Messages   []toolChatMessage  `json:"messages"`
	Tools      []toolChatToolSpec `json:"tools"`
	ToolChoice string             `json:"tool_choice"`
}

type toolChatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

type toolChatToolSpec struct {
	Type     string `json:"type"`
	Function struct {
		Name       string `json:"name"`
		Parameters struct {
			Required []string `json:"required"`
		} `json:"parameters"`
	} `json:"function"`
}
