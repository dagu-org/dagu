// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"bytes"
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
	"github.com/goccy/go-yaml"
)

var _ executor.Executor = (*harnessExecutor)(nil)
var _ executor.ExitCoder = (*harnessExecutor)(nil)

type providerConfig struct {
	provider Provider
	flags    map[string]any
}

type harnessExecutor struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdout     io.Writer
	stderr     io.Writer
	exitCode   int
	stderrTail *executor.TailWriter
	configs    []providerConfig
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
	var lastErr error

	for i, cfg := range e.configs {
		stdout, err := e.runOnce(ctx, cfg)
		if err == nil {
			e.exitCode = 0
			if len(stdout) > 0 {
				if _, writeErr := e.stdoutWriter().Write(stdout); writeErr != nil {
					e.exitCode = 1
					return fmt.Errorf("harness: failed to write stdout: %w", writeErr)
				}
			}
			return nil
		}

		lastErr = err
		if ctx.Err() != nil {
			return err
		}
		if i+1 < len(e.configs) {
			next := e.configs[i+1]
			_, _ = fmt.Fprintf(
				e.stderrWriter(),
				"harness: attempt %d/%d with %s failed; trying fallback %d/%d with %s\n",
				i+1,
				len(e.configs),
				cfg.provider.Name(),
				i+2,
				len(e.configs),
				next.provider.Name(),
			)
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return nil
}

func (e *harnessExecutor) runOnce(ctx context.Context, cfg providerConfig) ([]byte, error) {
	e.mu.Lock()

	env := runtime.GetEnv(ctx)
	tw := executor.NewTailWriterWithEncoding(e.stderrWriter(), 0, env.LogEncodingCharset)
	e.stderrTail = tw

	var stdout bytes.Buffer
	args := cfg.provider.BaseArgs(e.prompt)
	args = append(args, configToFlags(cfg.flags)...)

	cmd := exec.CommandContext(ctx, cfg.provider.BinaryName(), args...)
	cmd.Env = append(cmd.Env, runtime.AllEnvs(ctx)...)
	cmd.Dir = e.workDir
	cmd.Stdout = &stdout
	cmd.Stderr = tw
	cmdutil.SetupCommand(cmd)

	if cmd.Dir != "" {
		if err := os.MkdirAll(cmd.Dir, 0o750); err != nil {
			e.exitCode = 1
			e.mu.Unlock()
			return nil, fmt.Errorf("harness: failed to create working directory: %w", err)
		}
	}

	if e.script != "" {
		cmd.Stdin = strings.NewReader(e.script)
	}

	e.cmd = cmd

	if err := cmd.Start(); err != nil {
		e.exitCode = exitCodeFromError(err)
		e.mu.Unlock()
		if tail := tw.Tail(); tail != "" {
			return nil, fmt.Errorf("%w\nrecent stderr:\n%s", err, tail)
		}
		return nil, err
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
		return nil, ctx.Err()
	case err := <-waitDone:
		if err != nil {
			e.exitCode = exitCodeFromError(err)
			if tail := tw.Tail(); tail != "" {
				return nil, fmt.Errorf("%w\nrecent stderr:\n%s", err, tail)
			}
			return nil, err
		}
		return stdout.Bytes(), nil
	}
}

func (e *harnessExecutor) stdoutWriter() io.Writer {
	if e.stdout == nil {
		return io.Discard
	}
	return e.stdout
}

func (e *harnessExecutor) stderrWriter() io.Writer {
	if e.stderr == nil {
		return io.Discard
	}
	return e.stderr
}

// reservedKeys are config keys consumed by the harness executor itself, not passed as CLI flags.
var reservedKeys = map[string]bool{
	"provider":    true,
	"binary":      true,
	"prompt_args": true,
	"fallback":    true,
}

// configToFlags converts config map entries into CLI flags.
// Keys become --key, values are type-dependent:
//   - string → --key value
//   - bool true → --key (false is omitted)
//   - number → --key N
//   - []any → --key v1 --key v2 (repeated)
//
// Reserved keys are skipped. Keys are sorted for deterministic output.
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
		case int8:
			args = append(args, flag, strconv.FormatInt(int64(v), 10))
		case int16:
			args = append(args, flag, strconv.FormatInt(int64(v), 10))
		case int32:
			args = append(args, flag, strconv.FormatInt(int64(v), 10))
		case int64:
			args = append(args, flag, strconv.FormatInt(v, 10))
		case uint:
			args = append(args, flag, strconv.FormatUint(uint64(v), 10))
		case uint8:
			args = append(args, flag, strconv.FormatUint(uint64(v), 10))
		case uint16:
			args = append(args, flag, strconv.FormatUint(uint64(v), 10))
		case uint32:
			args = append(args, flag, strconv.FormatUint(uint64(v), 10))
		case uint64:
			args = append(args, flag, strconv.FormatUint(v, 10))
		case float32:
			args = append(args, flag, strconv.FormatFloat(float64(v), 'f', -1, 32))
		case float64:
			if v == float64(int(v)) {
				args = append(args, flag, strconv.Itoa(int(v)))
			} else {
				args = append(args, flag, strconv.FormatFloat(v, 'f', -1, 64))
			}
		case []string:
			for _, item := range v {
				args = append(args, flag, item)
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
	cfg := normalizeConfigMap(step.ExecutorConfig.Config)
	configs, err := buildProviderConfigs(cfg)
	if err != nil {
		return nil, err
	}

	prompt := extractPrompt(step)
	if prompt == "" {
		return nil, fmt.Errorf("harness: command field (prompt) is required")
	}

	env := runtime.GetEnv(ctx)

	return &harnessExecutor{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		configs: configs,
		prompt:  prompt,
		script:  step.Script,
		workDir: env.WorkingDir,
	}, nil
}

func buildProviderConfigs(cfg map[string]any) ([]providerConfig, error) {
	primary, fallbacks, err := extractFallbackConfigs(cfg)
	if err != nil {
		return nil, err
	}

	attempts := make([]map[string]any, 0, 1+len(fallbacks))
	attempts = append(attempts, primary)
	attempts = append(attempts, fallbacks...)

	configs := make([]providerConfig, 0, len(attempts))
	for i := range attempts {
		provider, err := resolveProvider(attempts[i])
		if err != nil {
			if i == 0 {
				return nil, err
			}
			return nil, fmt.Errorf("harness: invalid fallback[%d]: %w", i-1, err)
		}
		if _, err := exec.LookPath(provider.BinaryName()); err != nil {
			if i == 0 {
				return nil, fmt.Errorf("harness: %q CLI not found in PATH; install it first: %w", provider.BinaryName(), err)
			}
			return nil, fmt.Errorf("harness: fallback[%d] %q CLI not found in PATH; install it first: %w", i-1, provider.BinaryName(), err)
		}
		configs = append(configs, providerConfig{
			provider: provider,
			flags:    attempts[i],
		})
	}

	return configs, nil
}

func extractFallbackConfigs(cfg map[string]any) (map[string]any, []map[string]any, error) {
	primary := cloneConfigMap(cfg)
	raw, ok := primary["fallback"]
	if !ok {
		return primary, nil, nil
	}
	delete(primary, "fallback")

	fallbacks, err := fallbackConfigsFromValue(raw)
	if err != nil {
		return nil, nil, err
	}
	return primary, fallbacks, nil
}

func fallbackConfigsFromValue(raw any) ([]map[string]any, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []map[string]any:
		return cloneFallbackConfigs(v), nil
	case []any:
		fallbacks := make([]map[string]any, len(v))
		for i := range v {
			item, ok := v[i].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("harness: fallback[%d] must be an object", i)
			}
			fallbacks[i] = cloneConfigMap(item)
		}
		return fallbacks, nil
	default:
		return nil, fmt.Errorf("harness: fallback must be an array of objects")
	}
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
		if isTemplatedValue(providerName) {
			return nil, fmt.Errorf("harness: unresolved provider template %q", providerName)
		}
		return getProvider(providerName)
	case binaryName != "":
		if isTemplatedValue(binaryName) {
			return nil, fmt.Errorf("harness: unresolved binary template %q", binaryName)
		}
		promptArgs, err := promptArgsFromConfig(cfg["prompt_args"])
		if err != nil {
			return nil, err
		}
		return &customProvider{binary: binaryName, promptArgs: promptArgs}, nil
	default:
		return nil, fmt.Errorf("harness: config.provider or config.binary is required")
	}
}

func promptArgsFromConfig(raw any) ([]string, error) {
	if raw == nil {
		return []string{"-p"}, nil
	}

	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case []any:
		args := make([]string, len(v))
		for i := range v {
			args[i] = fmt.Sprint(v[i])
		}
		return args, nil
	default:
		return nil, fmt.Errorf("harness: config.prompt_args must be an array")
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

func normalizeConfigMap(cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}

	normalized := make(map[string]any, len(cfg))
	for key, value := range cfg {
		normalized[key] = normalizeConfigValue(value)
	}
	return normalized
}

func normalizeConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return normalizeConfigMap(v)
	case []any:
		normalized := make([]any, len(v))
		for i := range v {
			normalized[i] = normalizeConfigValue(v[i])
		}
		return normalized
	case []string:
		normalized := make([]string, len(v))
		for i := range v {
			normalized[i] = fmt.Sprint(coerceScalarString(v[i]))
		}
		return normalized
	case []map[string]any:
		normalized := make([]map[string]any, len(v))
		for i := range v {
			normalized[i] = normalizeConfigMap(v[i])
		}
		return normalized
	case string:
		return coerceScalarString(v)
	default:
		return value
	}
}

func coerceScalarString(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}

	var parsed any
	if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return value
	}

	switch parsed.(type) {
	case bool, int, int64, uint64, float32, float64:
		return parsed
	default:
		return value
	}
}

func cloneConfigMap(cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}

	cloned := make(map[string]any, len(cfg))
	for key, value := range cfg {
		cloned[key] = cloneConfigValue(value)
	}
	return cloned
}

func cloneFallbackConfigs(cfgs []map[string]any) []map[string]any {
	if cfgs == nil {
		return nil
	}

	cloned := make([]map[string]any, len(cfgs))
	for i := range cfgs {
		cloned[i] = cloneConfigMap(cfgs[i])
	}
	return cloned
}

func cloneConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneConfigMap(v)
	case []any:
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneConfigValue(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []map[string]any:
		return cloneFallbackConfigs(v)
	default:
		return value
	}
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

	if err := validateProviderConfig(cfg); err != nil {
		return err
	}

	fallbacks, err := fallbackConfigsFromValue(cfg["fallback"])
	if err != nil {
		return err
	}
	for i := range fallbacks {
		if err := validateProviderConfig(fallbacks[i]); err != nil {
			return fmt.Errorf("harness: invalid fallback[%d]: %w", i, err)
		}
	}

	return nil
}

func validateProviderConfig(cfg map[string]any) error {
	providerStr, _ := cfg["provider"].(string)
	binaryStr, _ := cfg["binary"].(string)
	if providerStr == "" && binaryStr == "" {
		return fmt.Errorf("harness: config.provider or config.binary is required")
	}
	if providerStr != "" && binaryStr != "" {
		return fmt.Errorf("harness: specify either provider or binary, not both")
	}
	if providerStr != "" && !isTemplatedValue(providerStr) {
		if _, err := getProvider(providerStr); err != nil {
			return err
		}
	}
	return nil
}

func isTemplatedValue(value string) bool {
	return strings.Contains(value, "${")
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
