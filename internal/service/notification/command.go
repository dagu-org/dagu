// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package notification

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	notificationmodel "github.com/dagucloud/dagu/v2/internal/notification"
	"github.com/dagucloud/dagu/v2/internal/service/chatbridge"
)

const (
	defaultNotificationCommandTimeout = 30 * time.Second
	commandStderrLimit                = 4 << 10
)

type commandEnvField struct {
	env   string
	token string
}

// commandEnvFields are the shell-safe names for notification template fields.
// Dotted template names such as dag.name are not expandable as $dag.name.
var commandEnvFields = []commandEnvField{
	{"DAG_NAME", "dag.name"},
	{"DAG_RUN_ID", "run.id"},
	{"STATUS", "run.status"},
	{"EVENT_TYPE", "event.type"},
	{"ERROR", "run.error"},
	{"WORKSPACE", "workspace"},
	{"WORKER_ID", "worker.id"},
	{"ATTEMPT_ID", "run.attemptId"},
	{"STARTED_AT", "run.startedAt"},
	{"FINISHED_AT", "run.finishedAt"},
	{"OBSERVED_AT", "event.observedAt"},
	{"RUN_PATH", "run.path"},
	{"RUN_URL", "run.url"},
	{"RUN_LINK", "run.link"},
	{"FAILED_STEPS", "run.failed_steps"},
	{"PARTIALLY_SUCCEEDED_STEPS", "run.partially_succeeded_steps"},
	{"ABORTED_STEPS", "run.aborted_steps"},
	{"SUCCEEDED_STEPS", "run.succeeded_steps"},
}

func (s *Service) sendCommand(ctx context.Context, target notificationmodel.Target, events []chatbridge.NotificationEvent) error {
	if target.Command == nil || strings.TrimSpace(target.Command.Command) == "" {
		return errors.New("command is not configured")
	}
	publicURL := s.publicURL()
	for _, event := range events {
		if event.Status == nil {
			continue
		}
		if err := s.runNotificationCommand(ctx, target.Command.Command, event, publicURL); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runNotificationCommand(
	ctx context.Context,
	command string,
	event chatbridge.NotificationEvent,
	publicURL string,
) error {
	rendered := renderNotificationTemplate(command, event, publicURL)
	if strings.TrimSpace(rendered) == "" {
		return errors.New("command is empty")
	}
	logPath, err := writeExecutionLog(event.Status)
	if err != nil {
		return err
	}
	if logPath != "" {
		defer os.Remove(logPath)
	}

	timeout := s.notificationCommandTimeout
	if timeout <= 0 {
		timeout = defaultNotificationCommandTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, prefix := notificationShell()
	args := append(append([]string{}, prefix...), rendered)
	cmd := exec.CommandContext(cmdCtx, shell, args...)
	configureNotificationCommand(cmd)
	cmd.Env = commandEnvironment(os.Environ(), notificationTemplateValues(event, publicURL), logPath)
	cmd.Stdout = io.Discard
	stderr := &limitedBuffer{remaining: commandStderrLimit}
	cmd.Stderr = stderr

	err = cmd.Run()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("command timed out after %s", timeout)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		detail := stderr.String()
		if detail != "" {
			return fmt.Errorf("command exited with status %d: %s", exitErr.ExitCode(), detail)
		}
		return fmt.Errorf("command exited with status %d", exitErr.ExitCode())
	}
	return fmt.Errorf("run notification command: %w", err)
}

func notificationShell() (string, []string) {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("ComSpec")
		if shell == "" {
			shell = `C:\Windows\System32\cmd.exe`
		}
		return shell, []string{"/c"}
	}
	return "/bin/sh", []string{"-c"}
}

func commandEnvironment(base []string, values map[string]string, logPath string) []string {
	extra := make(map[string]string, len(commandEnvFields)+1)
	for _, field := range commandEnvFields {
		extra[field.env] = sanitizeEnvValue(values[field.token])
	}
	extra["EXECUTION_LOG"] = sanitizeEnvValue(logPath)
	skip := make(map[string]struct{}, len(extra))
	for key := range extra {
		skip[envLookupKey(key)] = struct{}{}
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, found := skip[envLookupKey(key)]; found {
			continue
		}
		out = append(out, entry)
	}
	for _, key := range slices.Sorted(maps.Keys(extra)) {
		out = append(out, key+"="+extra[key])
	}
	return out
}

func envLookupKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func sanitizeEnvValue(value string) string {
	return strings.ReplaceAll(value, "\x00", "")
}

type commandLogSection struct {
	label string
	path  string
}

func executionLogSections(status *ir.DAGRunStatus) []commandLogSection {
	if status == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var sections []commandLogSection
	add := func(label, path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		sections = append(sections, commandLogSection{label: label, path: path})
	}
	add("DAG log", status.Log)
	for _, node := range status.Nodes {
		if node == nil {
			continue
		}
		name := node.Step.Name
		if name == "" {
			name = "step"
		}
		add("stdout "+name, node.Stdout)
		add("stderr "+name, node.Stderr)
	}
	return sections
}

func writeExecutionLog(status *ir.DAGRunStatus) (string, error) {
	sections := executionLogSections(status)
	if len(sections) == 0 {
		return "", nil
	}
	file, err := os.CreateTemp("", "dagu-notification-log-*")
	if err != nil {
		return "", fmt.Errorf("create notification log file: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	wrote := false
	for _, section := range sections {
		src, err := os.Open(section.path)
		if err != nil {
			continue
		}
		if wrote {
			if _, err := io.WriteString(file, "\n"); err != nil {
				src.Close()
				cleanup()
				return "", err
			}
		}
		if _, err := fmt.Fprintf(file, "# %s\n", section.label); err != nil {
			src.Close()
			cleanup()
			return "", err
		}
		if _, err := io.Copy(file, src); err != nil {
			src.Close()
			cleanup()
			return "", err
		}
		src.Close()
		wrote = true
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if !wrote {
		_ = os.Remove(path)
		return "", nil
	}
	return path, nil
}

type limitedBuffer struct {
	buf       bytes.Buffer
	remaining int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > b.remaining {
		_, _ = b.buf.Write(p[:b.remaining])
		b.remaining = 0
		b.truncated = true
		return len(p), nil
	}
	n, _ := b.buf.Write(p)
	b.remaining -= n
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	text := strings.TrimSpace(b.buf.String())
	if b.truncated && text != "" {
		return text + "..."
	}
	return text
}
