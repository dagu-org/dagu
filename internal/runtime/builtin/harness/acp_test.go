// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/stretchr/testify/require"
)

// TestACPAgent provides an independent JSON-RPC peer to check SDK wire behavior.
func TestACPAgent(_ *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "--acp-test-agent" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	mode := os.Args[marker+1]
	send := func(value any) { _ = json.NewEncoder(os.Stdout).Encode(value) }
	scanner := bufio.NewScanner(os.Stdin)
	var promptID any
	for scanner.Scan() {
		var msg struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			os.Exit(2)
		}
		respond := func(result any) { send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result}) }
		finish := func(reason string) {
			send(map[string]any{"jsonrpc": "2.0", "id": promptID, "result": map[string]any{"stopReason": reason}})
		}
		switch msg.Method {
		case "initialize":
			version := 1
			if mode == "version" {
				version = 999
			}
			respond(map[string]any{"protocolVersion": version, "agentCapabilities": map[string]any{}})
		case "session/new":
			var params struct{ Cwd string }
			_ = json.Unmarshal(msg.Params, &params)
			if !filepath.IsAbs(params.Cwd) {
				os.Exit(3)
			}
			respond(map[string]any{"sessionId": "test-session"})
			if mode == "blocked" {
				time.Sleep(time.Minute)
			}
		case "session/prompt":
			promptID = msg.ID
			if mode == "crash" {
				_, _ = os.Stderr.WriteString("adapter authentication failed\n")
				os.Exit(7)
			}
			if mode == "wait" {
				continue
			}
			if mode == "permission" {
				send(map[string]any{"jsonrpc": "2.0", "id": "permission-1", "method": "session/request_permission", "params": map[string]any{
					"sessionId": "test-session", "toolCall": map[string]any{"toolCallId": "tool-1"},
					"options": []any{map[string]any{"optionId": "once", "name": "Allow", "kind": "allow_once"}},
				}})
				continue
			}
			var params struct {
				Prompt []struct{ Text string }
			}
			_ = json.Unmarshal(msg.Params, &params)
			if len(params.Prompt) == 0 {
				os.Exit(4)
			}
			for _, part := range []string{"reply:", params.Prompt[0].Text} {
				send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
					"sessionId": "test-session", "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": part}},
				}})
			}
			reason := "end_turn"
			if mode == "refuse" {
				reason = "refusal"
			}
			finish(reason)
		case "session/cancel":
			finish("cancelled")
		default:
			if msg.ID == "permission-1" {
				if !strings.Contains(string(msg.Result), "selected") && !strings.Contains(string(msg.Result), "cancelled") {
					os.Exit(4)
				}
				finish("end_turn")
			}
		}
	}
	os.Exit(0)
}

// TestACPRun checks ACP output, permission decisions, failures, and cancellation.
func TestACPRun(t *testing.T) {
	for _, tc := range []struct {
		mode, policy, wantError string
	}{
		{mode: "echo"},
		{mode: "permission", wantError: "permission request denied"},
		{mode: "permission", policy: "allow"},
		{mode: "echo", policy: "invalid", wantError: "acp_permission"},
		{mode: "version", wantError: "unsupported protocol version"},
		{mode: "crash", wantError: "recent stderr:\nadapter authentication failed"},
		{mode: "refuse", wantError: "refusal"},
		{mode: "wait", wantError: "deadline exceeded"},
		{mode: "blocked", wantError: "deadline exceeded"},
	} {
		t.Run(tc.mode+tc.policy, func(t *testing.T) {
			binary, err := os.Executable()
			require.NoError(t, err)
			cfg := map[string]any{"provider": "test"}
			if tc.policy != "" {
				cfg["acp_permission"] = tc.policy
			}
			step := ir.Step{ExecutorConfig: ir.ExecutorConfig{Config: cfg}, Commands: []ir.CommandEntry{{Command: "hello"}}, Script: "context"}
			if tc.mode == "blocked" {
				step.Commands[0].Command = strings.Repeat("x", 2<<20)
			}
			dag := &ir.DAG{Harnesses: ir.HarnessDefinitions{"test": {Protocol: "acp", Binary: binary, PrefixArgs: []string{"-test.run=^TestACPAgent$", "--", "--acp-test-agent", tc.mode}}}}
			ctx := newHarnessTestContext(t, dag, step)
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			exec, err := newHarness(ctx, step)
			require.NoError(t, err)
			var out, stderr bytes.Buffer
			exec.SetStdout(&out)
			exec.SetStderr(&stderr)
			err = exec.Run(ctx)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError, stderr.String())
				require.Empty(t, out.String())
				return
			}
			require.NoError(t, err, stderr.String())
			if tc.mode == "echo" {
				require.Equal(t, "reply:hello\n\ncontext", out.String())
			}
		})
	}
}

// TestACPContainer verifies that unsupported container execution fails explicitly.
func TestACPContainer(t *testing.T) {
	e := &harnessExecutor{step: ir.Step{Container: &ir.Container{}}}
	_, err := e.runACP(t.Context(), providerConfig{})
	require.ErrorContains(t, err, "not supported inside containers")
}

// TestACPFallback ensures failed attempts do not leak text into successful output.
func TestACPFallback(t *testing.T) {
	binary, err := os.Executable()
	require.NoError(t, err)
	dag := &ir.DAG{Harnesses: ir.HarnessDefinitions{}}
	for _, mode := range []string{"refuse", "echo"} {
		dag.Harnesses[mode] = &ir.HarnessDefinition{Protocol: "acp", Binary: binary, PrefixArgs: []string{"-test.run=^TestACPAgent$", "--", "--acp-test-agent", mode}}
	}
	step := ir.Step{ExecutorConfig: ir.ExecutorConfig{Config: map[string]any{"provider": "refuse", "fallback": []any{map[string]any{"provider": "echo"}}}}, Commands: []ir.CommandEntry{{Command: "hello"}}}
	exec, err := newHarness(newHarnessTestContext(t, dag, step), step)
	require.NoError(t, err)
	var out, stderr bytes.Buffer
	exec.SetStdout(&out)
	exec.SetStderr(&stderr)
	require.NoError(t, exec.Run(t.Context()))
	require.Equal(t, "reply:hello", out.String())
	require.Contains(t, stderr.String(), "trying fallback")
}

// TestACPStop verifies that an explicit stop cancels the turn without trying a fallback.
func TestACPStop(t *testing.T) {
	binary, err := os.Executable()
	require.NoError(t, err)
	dag := &ir.DAG{Harnesses: ir.HarnessDefinitions{"agent": {Protocol: "acp", Binary: binary, PrefixArgs: []string{"-test.run=^TestACPAgent$", "--", "--acp-test-agent", "wait"}}}}
	step := ir.Step{ExecutorConfig: ir.ExecutorConfig{Config: map[string]any{"provider": "agent", "fallback": []any{map[string]any{"provider": "agent"}}}}, Commands: []ir.CommandEntry{{Command: "hello"}}}
	ctx := newHarnessTestContext(t, dag, step)
	exec, err := newHarness(ctx, step)
	require.NoError(t, err)
	e := exec.(*harnessExecutor)
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()
	require.Eventually(t, func() bool {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.acpCancel != nil
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, e.Stop(cmdutil.ForceTermination()))
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 137, e.ExitCode())
	case <-time.After(5 * time.Second):
		t.Fatal("ACP process did not stop")
	}
}

func TestACPProgress(t *testing.T) {
	binary, err := os.Executable()
	require.NoError(t, err)
	dag := &ir.DAG{Harnesses: ir.HarnessDefinitions{"agent": {Protocol: "acp", Binary: binary, PrefixArgs: []string{"-test.run=^TestACPAgent$", "--", "--acp-test-agent", "echo"}}}}
	step := ir.Step{ExecutorConfig: ir.ExecutorConfig{Config: map[string]any{"provider": "agent"}}, Commands: []ir.CommandEntry{{Command: "hello"}}}
	ctx := newHarnessTestContext(t, dag, step)
	exec, err := newHarness(ctx, step)
	require.NoError(t, err)
	var progress atomic.Int32
	exec.(*harnessExecutor).SetProgressCallback(func() { progress.Add(1) })
	require.NoError(t, exec.Run(ctx))
	require.Positive(t, progress.Load())
}

// Unknown transports must fail instead of silently starting a stdio agent.
func TestACPTransport(t *testing.T) {
	e := &harnessExecutor{}
	_, err := e.runACP(t.Context(), providerConfig{definition: &ir.HarnessDefinition{Protocol: ir.HarnessProtocolACP, Transport: "unknown"}})
	require.ErrorContains(t, err, "unsupported ACP transport")
}
