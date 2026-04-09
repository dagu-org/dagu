// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/dagucloud/dagu/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*harnessExecutor)(nil)
var _ executor.ExitCoder = (*harnessExecutor)(nil)

type harnessExecutor struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdout     io.Writer
	stderr     io.Writer
	exitCode   int
	stderrTail *executor.TailWriter
	provider   Provider
	config     map[string]any
	prompt     string
	script     string // piped to stdin if present
	workDir    string
}

func (e *harnessExecutor) ExitCode() int {
	return e.exitCode
}

func (e *harnessExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *harnessExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *harnessExecutor) Kill(sig os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return cmdutil.KillProcessGroup(e.cmd, sig)
}

func (e *harnessExecutor) Run(ctx context.Context) error {
	e.mu.Lock()

	env := runtime.GetEnv(ctx)

	tw := executor.NewTailWriterWithEncoding(e.stderr, 0, env.LogEncodingCharset)
	e.stderrTail = tw

	args := e.provider.BaseArgs(e.prompt)
	args = append(args, configToFlags(e.config)...)

	cmd := exec.CommandContext(ctx, e.provider.BinaryName(), args...)
	cmd.Env = append(cmd.Env, runtime.AllEnvs(ctx)...)
	cmd.Dir = e.workDir
	cmd.Stdout = e.stdout
	cmd.Stderr = tw
	cmdutil.SetupCommand(cmd)

	if e.script != "" {
		cmd.Stdin = strings.NewReader(e.script)
	}

	e.cmd = cmd

	if err := cmd.Start(); err != nil {
		e.exitCode = exitCodeFromError(err)
		e.mu.Unlock()
		if tail := tw.Tail(); tail != "" {
			return fmt.Errorf("%w\nrecent stderr:\n%s", err, tail)
		}
		return err
	}
	e.mu.Unlock()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = e.Kill(os.Kill)
		<-waitDone
		e.exitCode = 124
		return ctx.Err()
	case err := <-waitDone:
		if err != nil {
			e.exitCode = exitCodeFromError(err)
			if tail := tw.Tail(); tail != "" {
				return fmt.Errorf("%w\nrecent stderr:\n%s", err, tail)
			}
			return err
		}
		return nil
	}
}

// reservedKeys are config keys consumed by the harness executor itself, not passed as CLI flags.
var reservedKeys = map[string]bool{
	"provider":    true,
	"binary":      true,
	"prompt_args": true,
}

// configToFlags converts config map entries into CLI flags.
// Keys become --key, values are type-dependent:
//   - string → --key value
//   - bool true → --key (false is omitted)
//   - number → --key N
//   - []any → --key v1 --key v2 (repeated)
//
// Reserved keys (provider, binary, prompt_args) are skipped.
// Keys are sorted for deterministic output.
func configToFlags(cfg map[string]any) []string {
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		if reservedKeys[k] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var args []string
	for _, key := range keys {
		flag := "--" + key
		switch v := cfg[key].(type) {
		case bool:
			if v {
				args = append(args, flag)
			}
		case string:
			if v != "" {
				args = append(args, flag, v)
			}
		case int:
			args = append(args, flag, strconv.Itoa(v))
		case float64:
			if v == float64(int(v)) {
				args = append(args, flag, strconv.Itoa(int(v)))
			} else {
				args = append(args, flag, strconv.FormatFloat(v, 'f', -1, 64))
			}
		case []any:
			for _, item := range v {
				args = append(args, flag, fmt.Sprint(item))
			}
		}
	}
	return args
}

func newHarness(ctx context.Context, step core.Step) (executor.Executor, error) {
	cfg := step.ExecutorConfig.Config

	provider, err := resolveProvider(cfg)
	if err != nil {
		return nil, err
	}

	if _, err := exec.LookPath(provider.BinaryName()); err != nil {
		return nil, fmt.Errorf("harness: %q CLI not found in PATH; install it first: %w", provider.BinaryName(), err)
	}

	prompt := extractPrompt(step)
	if prompt == "" {
		return nil, fmt.Errorf("harness: command field (prompt) is required")
	}

	env := runtime.GetEnv(ctx)

	return &harnessExecutor{
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		provider: provider,
		config:   cfg,
		prompt:   prompt,
		script:   step.Script,
		workDir:  env.WorkingDir,
	}, nil
}

// resolveProvider returns a Provider from either a built-in name or a custom binary definition.
//
// Built-in: config.provider = "claude" (uses registered provider)
// Custom:   config.binary = "gemini", config.prompt_args = ["-p"] (user-defined)
//
// prompt_args defines the base CLI arguments for passing the prompt. The prompt
// string is appended after these args. For example, prompt_args: ["-p"] produces
// ["gemini", "-p", "<prompt>", ...flags]. Defaults to ["-p"] if omitted.
func resolveProvider(cfg map[string]any) (Provider, error) {
	providerName, _ := cfg["provider"].(string)
	binaryName, _ := cfg["binary"].(string)

	switch {
	case providerName != "" && binaryName != "":
		return nil, fmt.Errorf("harness: specify either provider or binary, not both")
	case providerName != "":
		return getProvider(providerName)
	case binaryName != "":
		promptArgs := []string{"-p"}
		if raw, ok := cfg["prompt_args"]; ok {
			if arr, ok := raw.([]any); ok {
				promptArgs = make([]string, len(arr))
				for i, v := range arr {
					promptArgs[i] = fmt.Sprint(v)
				}
			}
		}
		return &customProvider{binary: binaryName, promptArgs: promptArgs}, nil
	default:
		return nil, fmt.Errorf("harness: config.provider or config.binary is required")
	}
}

// customProvider is a user-defined provider specified via config.binary and config.prompt_args.
type customProvider struct {
	binary     string
	promptArgs []string
}

func (p *customProvider) Name() string       { return p.binary }
func (p *customProvider) BinaryName() string { return p.binary }

func (p *customProvider) BaseArgs(prompt string) []string {
	args := make([]string, len(p.promptArgs))
	copy(args, p.promptArgs)
	return append(args, prompt)
}

func extractPrompt(step core.Step) string {
	if len(step.Commands) == 0 {
		return ""
	}
	cmd := step.Commands[0]
	if cmd.CmdWithArgs != "" {
		return cmd.CmdWithArgs
	}
	if cmd.Command == "" {
		return ""
	}
	if len(cmd.Args) > 0 {
		return cmd.Command + " " + strings.Join(cmd.Args, " ")
	}
	return cmd.Command
}

func validateHarnessStep(step core.Step) error {
	if len(step.Commands) == 0 || extractPrompt(step) == "" {
		return fmt.Errorf("harness: command field (prompt) is required")
	}
	cfg := step.ExecutorConfig.Config
	if cfg == nil {
		return fmt.Errorf("harness: config is required")
	}
	providerStr, _ := cfg["provider"].(string)
	binaryStr, _ := cfg["binary"].(string)
	if providerStr == "" && binaryStr == "" {
		return fmt.Errorf("harness: config.provider or config.binary is required")
	}
	if providerStr != "" && binaryStr != "" {
		return fmt.Errorf("harness: specify either provider or binary, not both")
	}
	if providerStr != "" {
		if _, err := getProvider(providerStr); err != nil {
			return err
		}
	}
	return nil
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func init() {
	caps := core.ExecutorCapabilities{
		Command: true,
		Script:  true,
	}
	executor.RegisterExecutor("harness", newHarness, validateHarnessStep, caps)
}
