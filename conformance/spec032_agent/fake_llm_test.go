// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec032_agent_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// scriptedTurn is one canned reply from the fake OpenAI-compatible chat
// completions endpoint. An agent turn is a strict request/response cycle, so
// a call-order-indexed script is sufficient to drive a deterministic decision
// loop without needing to parse the actual conversation each fake server
// receives.
type scriptedTurn struct {
	// tool is the function name to call. Empty means a plain reply with no
	// tool call (used to test stalling).
	tool string
	// args is the JSON-encoded arguments string for the tool call.
	args string
	// content is the assistant's text content.
	content string
}

type fakeToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type fakeToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function fakeToolCallFunction `json:"function"`
}

type fakeMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []fakeToolCall `json:"tool_calls,omitempty"`
}

type fakeChoice struct {
	Index        int         `json:"index"`
	Message      fakeMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type fakeUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type fakeChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []fakeChoice `json:"choices"`
	Usage   fakeUsage    `json:"usage"`
}

// fakeLLM is a minimal OpenAI-compatible chat-completions server driving the
// "local" LLM provider, so the agent decision loop can be exercised
// end-to-end through the real HTTP wire format without a live API key.
type fakeLLM struct {
	mu     sync.Mutex
	turns  []scriptedTurn
	calls  int
	server *httptest.Server
}

func newFakeLLM(t *testing.T, turns []scriptedTurn) *fakeLLM {
	t.Helper()

	f := &fakeLLM{turns: turns}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeLLM) url() string {
	return f.server.URL
}

// env returns the DAGU_TEST_LLM_URL override the fixtures resolve
// llm.base_url against via ${env.DAGU_TEST_LLM_URL}.
func (f *fakeLLM) env() []string {
	return []string{"DAGU_TEST_LLM_URL=" + f.url()}
}

func (f *fakeLLM) handle(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	idx := f.calls
	f.calls++
	f.mu.Unlock()

	turn := scriptedTurn{content: "no more scripted turns"}
	if idx < len(f.turns) {
		turn = f.turns[idx]
	}

	msg := fakeMessage{Role: "assistant", Content: turn.content}
	finishReason := "stop"
	if turn.tool != "" {
		msg.ToolCalls = []fakeToolCall{{
			ID:   "call_" + strconv.Itoa(idx),
			Type: "function",
			Function: fakeToolCallFunction{
				Name:      turn.tool,
				Arguments: turn.args,
			},
		}}
		finishReason = "tool_calls"
	}

	resp := fakeChatResponse{
		ID:      "chatcmpl-fake-" + strconv.Itoa(idx),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "fake-model",
		Choices: []fakeChoice{{Index: 0, Message: msg, FinishReason: finishReason}},
		Usage:   fakeUsage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// setTaskStatusArgs builds the JSON arguments string for a set_task_status
// tool call.
func setTaskStatusArgs(task, status, reason string) string {
	b, err := json.Marshal(map[string]string{"task": task, "status": status, "reason": reason})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// askUserArgs builds the JSON arguments string for an ask_user tool call.
func askUserArgs(question string) string {
	b, err := json.Marshal(map[string]string{"question": question})
	if err != nil {
		panic(err)
	}
	return string(b)
}
