// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec067_harness_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestACPFixture(_ *testing.T) {
	if !slices.Contains(os.Args, "--acp-fixture") {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	send := func(v any) { _ = json.NewEncoder(os.Stdout).Encode(v) }
	for scanner.Scan() {
		var msg struct {
			ID     any
			Method string
			Params struct {
				Cwd    string
				Prompt []struct{ Text string }
			}
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			os.Exit(2)
		}
		result := map[string]any{}
		switch msg.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{}}
		case "session/new":
			if !filepath.IsAbs(msg.Params.Cwd) {
				os.Exit(3)
			}
			result["sessionId"] = "fixture"
		case "session/prompt":
			for _, part := range []string{"ACP:", msg.Params.Prompt[0].Text} {
				send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
					"sessionId": "fixture", "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": part}},
				}})
			}
			result["stopReason"] = "end_turn"
		}
		send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
	}
	os.Exit(0)
}

func TestHarnessACP(t *testing.T) {
	dagu := harness.NewRunner(t)
	binary, err := os.Executable()
	require.NoError(t, err)
	dagu.WriteFile("acp.yaml", fmt.Sprintf(`harnesses:
  adapter:
    protocol: acp
    binary: %q
    prefix_args: ["-test.run=^TestACPFixture$", "--", "--acp-fixture"]
steps:
  - id: ask
    action: harness.run
    with:
      provider: adapter
      prompt: hello
      stdin: context
    output: ANSWER
  - id: use
    depends: ask
    action: harness.run
    with:
      provider: adapter
      prompt: "${ANSWER}"
`, binary))
	dagu.Run("validate", "acp.yaml").ExpectExitCode(0)
	result := dagu.Run("start", "acp.yaml")
	result.ExpectExitCode(0)
	require.Equal(t, "ACP:hello\n\ncontext", stepStdout(t, result.Stdout(), 0))
	require.Equal(t, "ACP:ACP:hello\n\ncontext", stepStdout(t, result.Stdout(), 1))
}
