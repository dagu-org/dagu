// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/dagucloud/dagu/v2/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/runtime/resourcelimit"
)

// ACP 使用独立的双向 stdio 通道，不能复用只接收 CLI 文本的容器路径。
func (e *harnessExecutor) runACP(parent context.Context, cfg providerConfig) (stdout *os.File, err error) {
	e.exitCode = 1
	if e.step.Container != nil || rootContainerConfigured(parent) {
		return nil, errors.New("harness: ACP is not supported inside containers")
	}
	policy := "deny"
	flags := cloneConfigMap(cfg.flags)
	if raw, ok := flags["acp_permission"]; ok {
		policy, _ = raw.(string)
		delete(flags, "acp_permission")
	}
	if policy != "deny" && policy != "allow" {
		return nil, errors.New("harness: acp_permission must be deny or allow")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	spool, err := newStdoutSpool()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = cleanupStdoutSpool(spool)
		}
	}()
	args := append([]string(nil), cfg.definition.PrefixArgs...)
	args = append(args, configToFlags(flags, cfg.definition)...)
	cmd, err := e.invocationCommand(ctx, cfg, args, nil, nil, e.stderrWriter())
	if err != nil {
		return nil, err
	}
	cmd.Dir, err = filepath.Abs(cmd.Dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(cmd.Dir, 0o750); err != nil {
		return nil, err
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer func() { _ = input.Close() }()
	cmd.Stdout = nil
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	// WaitDelay 同时限制 agent 子进程持有 stderr 管道时的清理时间。
	cmd.WaitDelay = time.Second
	e.mu.Lock()
	process, err := cmdutil.StartManagedProcess(cmd)
	if err != nil {
		e.mu.Unlock()
		return nil, fmt.Errorf("harness: start ACP agent: %w", err)
	}
	if guard := resourcelimit.FromContext(ctx); guard != nil {
		if assignErr := guard.AssignProcess(process.PID()); assignErr != nil {
			logger.Warn(ctx, "Resource limits requested but process assignment failed", tag.Error(assignErr))
		}
	}
	e.process, e.acpCancel = process, cancel
	e.mu.Unlock()
	client := &acpHarnessClient{out: spool, allow: policy == "allow"}
	conn := acp.NewClientSideConnection(client, input, output)
	defer func() {
		_ = input.Close()
		_, _ = process.Stop(cmdutil.StopRequest{Intent: cmdutil.ForceTermination()})
		_ = process.Wait()
		_ = output.Close()
		<-conn.Done()
		_ = process.Release()
		e.mu.Lock()
		e.process, e.acpCancel = nil, nil
		e.mu.Unlock()
	}()
	// 即使 agent 忽略取消通知，也必须结束整个进程树。
	watchDone := make(chan struct{})
	stopWatch := context.AfterFunc(ctx, func() {
		defer close(watchDone)
		cancelCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		client.mu.Lock()
		session := client.session
		client.mu.Unlock()
		if session != "" {
			sent := make(chan struct{})
			go func() {
				defer close(sent)
				_ = conn.Cancel(cancelCtx, acp.CancelNotification{SessionId: session})
			}()
			select {
			case <-sent:
			case <-cancelCtx.Done():
			}
			// SDK 的管道写入不一定响应 context；关闭管道可解除阻塞。
			_ = input.Close()
			<-sent
		}
		_, _ = process.Stop(cmdutil.StopRequest{Intent: cmdutil.ForceTermination()})
		_ = input.Close()
	})
	defer func() {
		if !stopWatch() {
			<-watchDone
		}
	}()
	if err = e.acpTurn(ctx, conn, client, cmd.Dir); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("harness: ACP: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if _, err = spool.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return spool, nil
}

func (e *harnessExecutor) acpTurn(ctx context.Context, conn *acp.ClientSideConnection, client *acpHarnessClient, directory string) error {
	init, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if init.ProtocolVersion != acp.ProtocolVersionNumber {
		return fmt.Errorf("unsupported protocol version %d", init.ProtocolVersion)
	}
	session, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: directory, McpServers: []acp.McpServer{}})
	if err != nil {
		return fmt.Errorf("new session (authenticate the adapter before running Dagu): %w", err)
	}
	if session.SessionId == "" {
		return errors.New("agent returned an empty session ID")
	}
	client.mu.Lock()
	client.session = session.SessionId
	client.mu.Unlock()
	prompt := promptAndScript(e.effectivePrompt(), e.script)
	response, err := conn.Prompt(ctx, acp.PromptRequest{SessionId: session.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock(prompt)}})
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err != nil {
		return client.err
	}
	if response.StopReason != acp.StopReasonEndTurn {
		return fmt.Errorf("prompt stopped: %s", response.StopReason)
	}
	return nil
}

type acpHarnessClient struct {
	mu      sync.Mutex
	out     io.Writer
	allow   bool
	session acp.SessionId
	err     error
}

var _ acp.Client = (*acpHarnessClient)(nil)

func (c *acpHarnessClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if params.SessionId != c.session {
		return nil
	}
	if chunk := params.Update.AgentMessageChunk; chunk != nil && chunk.Content.Text != nil {
		_, err := io.WriteString(c.out, chunk.Content.Text.Text)
		if err != nil {
			c.err = err
		}
		return err
	}
	return nil
}

func (c *acpHarnessClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if params.SessionId == c.session && c.allow {
		for _, option := range params.Options {
			if option.Kind == acp.PermissionOptionKindAllowOnce {
				return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: option.OptionId}}}, nil
			}
		}
	}
	c.err = errors.New("ACP permission request denied; set acp_permission: allow to accept allow_once requests")
	return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}}}, nil
}

// 不声明客户端文件系统和终端能力；agent 可使用自己的工具实现。
func (*acpHarnessClient) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound("fs/read_text_file")
}

func (*acpHarnessClient) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound("fs/write_text_file")
}

func (*acpHarnessClient) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound("terminal")
}

func (*acpHarnessClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound("terminal")
}

func (*acpHarnessClient) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound("terminal")
}

func (*acpHarnessClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound("terminal")
}

func (*acpHarnessClient) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound("terminal")
}
