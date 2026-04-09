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
	"strings"
	"sync"

	"github.com/dagucloud/dagu/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/runtime/executor"
	"github.com/go-viper/mapstructure/v2"
)

var _ executor.Executor = (*harnessExecutor)(nil)
var _ executor.ExitCoder = (*harnessExecutor)(nil)

// harnessConfig holds the configuration decoded from step.ExecutorConfig.Config.
type harnessConfig struct {
	// Common fields
	Provider     string `mapstructure:"provider"`      // required: claude | codex | opencode | pi
	Model        string `mapstructure:"model"`          // provider-specific model name
	Effort       string `mapstructure:"effort"`         // low | medium | high | max
	MaxTurns     int    `mapstructure:"max_turns"`      // max agentic iterations
	OutputFormat string `mapstructure:"output_format"`  // text | json | stream-json

	// Claude-specific
	AllowedTools       string  `mapstructure:"allowed_tools"`
	DisallowedTools    string  `mapstructure:"disallowed_tools"`
	PermissionMode     string  `mapstructure:"permission_mode"`
	SystemPrompt       string  `mapstructure:"system_prompt"`
	AppendSystemPrompt string  `mapstructure:"append_system_prompt"`
	MaxBudgetUSD       float64 `mapstructure:"max_budget_usd"`
	Bare               bool    `mapstructure:"bare"`
	AddDir             string  `mapstructure:"add_dir"`
	Worktree           bool    `mapstructure:"worktree"`

	// Codex-specific
	Sandbox      string `mapstructure:"sandbox"`
	FullAuto     bool   `mapstructure:"full_auto"`
	OutputSchema string `mapstructure:"output_schema"`
	Ephemeral    bool   `mapstructure:"ephemeral"`
	SkipGitCheck bool   `mapstructure:"skip_git_repo_check"`

	// OpenCode-specific
	Agent string `mapstructure:"agent"`
	File  string `mapstructure:"file"`
	Title string `mapstructure:"title"`

	// Pi-specific
	Thinking     string `mapstructure:"thinking"`
	PiProvider   string `mapstructure:"pi_provider"`
	Tools        string `mapstructure:"tools"`
	NoTools      bool   `mapstructure:"no_tools"`
	NoExtensions bool   `mapstructure:"no_extensions"`
	Session      string `mapstructure:"session"`

	// Escape hatch for flags not yet modeled
	ExtraFlags []string `mapstructure:"extra_flags"`
}

type harnessExecutor struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdout     io.Writer
	stderr     io.Writer
	exitCode   int
	stderrTail *executor.TailWriter
	provider   Provider
	config     *harnessConfig
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

	args := e.provider.BuildArgs(e.config, e.prompt)

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

func newHarness(ctx context.Context, step core.Step) (executor.Executor, error) {
	var cfg harnessConfig
	if step.ExecutorConfig.Config != nil {
		if err := decodeConfig(step.ExecutorConfig.Config, &cfg); err != nil {
			return nil, fmt.Errorf("harness: invalid config: %w", err)
		}
	}

	if cfg.Provider == "" {
		return nil, fmt.Errorf("harness: config.provider is required (claude, codex, opencode, pi)")
	}

	provider, err := getProvider(cfg.Provider)
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
		config:   &cfg,
		prompt:   prompt,
		script:   step.Script,
		workDir:  env.WorkingDir,
	}, nil
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
	if len(step.Commands) == 0 {
		return fmt.Errorf("harness: command field (prompt) is required")
	}
	cfg := step.ExecutorConfig.Config
	if cfg == nil {
		return fmt.Errorf("harness: config is required")
	}
	provider, ok := cfg["provider"]
	if !ok || provider == "" {
		return fmt.Errorf("harness: config.provider is required (claude, codex, opencode, pi)")
	}
	providerStr, ok := provider.(string)
	if !ok {
		return fmt.Errorf("harness: config.provider must be a string")
	}
	if _, err := getProvider(providerStr); err != nil {
		return err
	}
	return nil
}

func decodeConfig(data map[string]any, cfg *harnessConfig) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	if err != nil {
		return err
	}
	return dec.Decode(data)
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
