// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/service/audit"
	"github.com/google/uuid"
)

const (
	defaultTerminalRows       = 24
	defaultTerminalCols       = 80
	auditTimeout              = 5 * time.Second
	processShutdownGrace      = 3 * time.Second
	processExitDetectionDelay = 250 * time.Millisecond
)

const (
	terminalEndReasonClosed      = "closed"
	terminalEndReasonClientClose = "client_close"
	terminalEndReasonDisconnect  = "disconnect"
	terminalEndReasonShutdown    = "shutdown"
	terminalEndReasonPTYError    = "pty_error"
	terminalEndReasonShellExit   = "shell_exit"
)

type runEvent struct {
	reason        string
	sendOutput    string
	sendError     string
	gracefulClose bool
	err           error
}

// Connection represents an interactive terminal connection.
type Connection struct {
	ID         string
	User       *auth.User
	IPAddress  string
	Shell      string
	Conn       *websocket.Conn
	StartTime  time.Time
	LastActive time.Time

	ptmx *os.File
	cmd  *exec.Cmd

	sendMu sync.Mutex
	state  sync.Mutex

	inputBuffer []byte // accumulates input until newline for command logging
	inEscSeq    bool   // true when processing an ANSI escape sequence
	closing     atomic.Bool
}

// NewConnection creates a new terminal connection.
func NewConnection(user *auth.User, shell string, conn *websocket.Conn, ipAddress string) *Connection {
	now := time.Now()
	return &Connection{
		ID:         uuid.New().String(),
		User:       user,
		IPAddress:  ipAddress,
		Shell:      shell,
		Conn:       conn,
		StartTime:  now,
		LastActive: now,
	}
}

// Run starts the terminal connection and handles communication between
// the WebSocket and the PTY.
func (c *Connection) Run(ctx context.Context, auditSvc *audit.Service) (retErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	c.logConnectionStart(auditSvc)

	cmd := exec.Command(c.Shell) //nolint:gosec // shell path is from config, not user input
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		c.sendError("Failed to start shell: " + err.Error())
		c.logConnectionEnd(auditSvc, terminalEndReasonPTYError)
		return err
	}

	c.cmd = cmd
	c.ptmx = ptmx
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: defaultTerminalRows, Cols: defaultTerminalCols})

	sessionCtx, cancel := context.WithCancel(ctx)
	var (
		ioWG        sync.WaitGroup
		processDone = make(chan struct{})
		eventCh     = make(chan runEvent, 1)
		eventOnce   sync.Once
		event       = runEvent{reason: terminalEndReasonClosed}
	)

	signalEvent := func(ev runEvent) {
		eventOnce.Do(func() {
			eventCh <- ev
		})
	}

	defer func() {
		cleanupErr := c.cleanup(cancel, &ioWG, processDone, event.gracefulClose)
		c.logConnectionEnd(auditSvc, event.reason)
		if cleanupErr != nil {
			if retErr != nil {
				retErr = errors.Join(retErr, cleanupErr)
			} else {
				retErr = cleanupErr
			}
		}
	}()

	ioWG.Add(2)
	go func() {
		defer ioWG.Done()
		c.readFromPTY(sessionCtx, processDone, signalEvent)
	}()
	go func() {
		defer ioWG.Done()
		c.readFromWebSocket(sessionCtx, auditSvc, signalEvent)
	}()
	go func() {
		waitErr := cmd.Wait()
		close(processDone)
		signalEvent(classifyProcessExit(waitErr))
	}()
	go func() {
		<-sessionCtx.Done()
		signalEvent(runEvent{
			reason: terminalEndReasonShutdown,
		})
	}()

	event = <-eventCh
	if event.sendError != "" {
		c.sendError(event.sendError)
	}
	if event.sendOutput != "" {
		c.sendOutput(event.sendOutput)
	}

	return event.err
}

// readFromPTY reads output from the PTY and sends it to the WebSocket.
func (c *Connection) readFromPTY(ctx context.Context, processDone <-chan struct{}, signal func(runEvent)) {
	buf := make([]byte, 4096)
	for {
		n, err := c.ptmx.Read(buf)
		if err != nil {
			if c.shouldSuppressPTYReadError(err, processDone) {
				return
			}
			signal(runEvent{
				reason:    terminalEndReasonPTYError,
				sendError: "Shell closed: " + err.Error(),
				err:       err,
			})
			return
		}

		if n == 0 {
			continue
		}

		writeCtx, cancel := context.WithTimeout(ctx, auditTimeout)
		err = c.sendMessage(writeCtx, NewOutputMessage(buf[:n]))
		cancel()
		if err != nil {
			if c.isClosing() || errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			signal(c.classifyWebSocketWriteError(ctx, err))
			return
		}

		c.updateLastActive()
	}
}

// readFromWebSocket reads input from the WebSocket and writes to the PTY.
func (c *Connection) readFromWebSocket(ctx context.Context, auditSvc *audit.Service, signal func(runEvent)) {
	for {
		_, data, err := c.Conn.Read(ctx)
		if err != nil {
			signal(c.classifyWebSocketError(ctx, err))
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		c.updateLastActive()

		switch msg.Type {
		case MessageTypeInput:
			input, err := msg.DecodeData()
			if err != nil {
				continue
			}
			if len(input) == 0 {
				continue
			}

			if _, err := c.ptmx.Write(input); err != nil {
				if c.isClosing() || errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				signal(runEvent{
					reason:    terminalEndReasonPTYError,
					sendError: "Failed to write to shell: " + err.Error(),
					err:       err,
				})
				return
			}

			if auditSvc != nil {
				c.accumulateAndLogCommand(auditSvc, input)
			}

		case MessageTypeResize:
			if msg.Cols > 0 && msg.Cols <= 500 && msg.Rows > 0 && msg.Rows <= 500 {
				if err := pty.Setsize(c.ptmx, &pty.Winsize{
					Rows: uint16(msg.Rows), //nolint:gosec // bounds checked above
					Cols: uint16(msg.Cols), //nolint:gosec // bounds checked above
				}); err != nil && !c.isClosing() {
					signal(runEvent{
						reason:    terminalEndReasonPTYError,
						sendError: "Failed to resize terminal: " + err.Error(),
						err:       err,
					})
					return
				}
			}

		case MessageTypeClose:
			signal(runEvent{reason: terminalEndReasonClientClose})
			return

		case MessageTypeOutput, MessageTypeError:
			// These are server-to-client message types, ignore if received from client.
		}
	}
}

// ForceKill expedites terminal teardown during hard server shutdown.
func (c *Connection) ForceKill() {
	c.closing.Store(true)
	if c.ptmx != nil {
		_ = c.ptmx.SetReadDeadline(time.Now())
	}
	if c.Conn != nil {
		_ = c.Conn.CloseNow()
	}
	_ = forceKillProcess(c.cmd)
}

func (c *Connection) cleanup(cancel context.CancelFunc, ioWG *sync.WaitGroup, processDone <-chan struct{}, gracefulClose bool) error {
	if !c.closing.CompareAndSwap(false, true) {
		return nil
	}

	cancel()
	if c.Conn != nil {
		if gracefulClose {
			_ = c.Conn.Close(websocket.StatusNormalClosure, "connection closed")
		} else {
			_ = c.Conn.CloseNow()
		}
	}
	if c.ptmx != nil {
		_ = c.ptmx.SetReadDeadline(time.Now())
	}

	ioWG.Wait()

	if c.ptmx != nil {
		_ = c.ptmx.Close()
	}

	return c.terminateProcess(processDone)
}

func (c *Connection) terminateProcess(processDone <-chan struct{}) error {
	if c.cmd == nil {
		return nil
	}
	if waitForSignal(processDone, 0) {
		return nil
	}

	var errs []error
	if err := requestHangup(c.cmd); err != nil {
		errs = append(errs, err)
	}
	if waitForSignal(processDone, processShutdownGrace) {
		return errors.Join(errs...)
	}

	if err := forceKillProcess(c.cmd); err != nil {
		errs = append(errs, err)
	}
	<-processDone
	return errors.Join(errs...)
}

func (c *Connection) classifyWebSocketError(ctx context.Context, err error) runEvent {
	if errors.Is(ctx.Err(), context.Canceled) {
		return runEvent{
			reason: terminalEndReasonShutdown,
		}
	}

	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return runEvent{
			reason: terminalEndReasonClientClose,
		}
	}
	if status == -1 {
		return runEvent{
			reason: terminalEndReasonDisconnect,
		}
	}
	return runEvent{
		reason: terminalEndReasonDisconnect,
	}
}

func (c *Connection) classifyWebSocketWriteError(ctx context.Context, err error) runEvent {
	if errors.Is(ctx.Err(), context.Canceled) {
		return runEvent{reason: terminalEndReasonShutdown}
	}

	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return runEvent{reason: terminalEndReasonClientClose}
	}
	if status == -1 {
		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
			return runEvent{reason: terminalEndReasonDisconnect}
		}
		return runEvent{
			reason: terminalEndReasonDisconnect,
			err:    err,
		}
	}
	return runEvent{reason: terminalEndReasonDisconnect}
}

func classifyProcessExit(err error) runEvent {
	if err == nil {
		return runEvent{
			reason:        terminalEndReasonShellExit,
			sendOutput:    "\r\nShell closed.\r\n",
			gracefulClose: true,
		}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return runEvent{
			reason:        terminalEndReasonShellExit,
			sendError:     "Shell closed: " + err.Error(),
			gracefulClose: true,
		}
	}

	return runEvent{
		reason:    terminalEndReasonPTYError,
		sendError: "Shell closed: " + err.Error(),
		err:       err,
	}
}

func (c *Connection) shouldSuppressPTYReadError(err error, processDone <-chan struct{}) bool {
	if c.isClosing() && isExpectedShutdownReadError(err) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return waitForSignal(processDone, processExitDetectionDelay)
	}
	if errors.Is(err, syscall.EIO) {
		return waitForSignal(processDone, processExitDetectionDelay)
	}
	return c.isClosing()
}

func isExpectedShutdownReadError(err error) bool {
	if errors.Is(err, os.ErrClosed) || errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func waitForSignal(ch <-chan struct{}, timeout time.Duration) bool {
	if timeout <= 0 {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

// sendMessage sends a message to the WebSocket.
func (c *Connection) sendMessage(ctx context.Context, msg *Message) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.isClosing() {
		return net.ErrClosed
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.Conn.Write(ctx, websocket.MessageText, data)
}

func (c *Connection) sendOutput(output string) {
	ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
	defer cancel()
	_ = c.sendMessage(ctx, NewOutputMessage([]byte(output)))
}

// sendError sends an error message to the WebSocket.
func (c *Connection) sendError(errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
	defer cancel()
	_ = c.sendMessage(ctx, NewErrorMessage(errMsg))
}

// updateLastActive updates the last active timestamp.
func (c *Connection) updateLastActive() {
	c.state.Lock()
	defer c.state.Unlock()
	c.LastActive = time.Now()
}

func (c *Connection) isClosing() bool {
	return c.closing.Load()
}

func (c *Connection) logConnectionStart(auditSvc *audit.Service) {
	if auditSvc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
	defer cancel()
	_ = auditSvc.LogTerminalConnectionStart(ctx, c.User.ID, c.User.Username, c.ID, c.IPAddress)
}

func (c *Connection) logConnectionEnd(auditSvc *audit.Service, reason string) {
	if auditSvc == nil {
		return
	}
	if reason == "" {
		reason = terminalEndReasonClosed
	}
	ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
	defer cancel()
	_ = auditSvc.LogTerminalConnectionEnd(ctx, c.User.ID, c.User.Username, c.ID, reason, c.IPAddress)
}

func (c *Connection) logCommand(auditSvc *audit.Service, command string) {
	if auditSvc == nil || command == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), auditTimeout)
	defer cancel()
	_ = auditSvc.LogTerminalCommand(ctx, c.User.ID, c.User.Username, c.ID, command, c.IPAddress)
}

// accumulateAndLogCommand accumulates input and logs complete commands when Enter is pressed.
func (c *Connection) accumulateAndLogCommand(auditSvc *audit.Service, input []byte) {
	for _, b := range input {
		// Handle ANSI escape sequences (e.g., arrow keys send ESC[A, ESC[B, etc.)
		if c.inEscSeq {
			// Escape sequences end with a letter (A-Z, a-z)
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				c.inEscSeq = false
			}
			continue
		}

		switch b {
		case 27: // ESC - start of escape sequence
			c.inEscSeq = true
		case '\r', '\n':
			// Enter pressed - log the accumulated command
			if len(c.inputBuffer) > 0 {
				c.logCommand(auditSvc, string(c.inputBuffer))
				c.inputBuffer = nil
			}
		case 127, '\b':
			// Backspace - remove last character from buffer
			if len(c.inputBuffer) > 0 {
				c.inputBuffer = c.inputBuffer[:len(c.inputBuffer)-1]
			}
		case 3:
			// Ctrl+C - clear the buffer
			c.inputBuffer = nil
		case 21:
			// Ctrl+U - clear the line
			c.inputBuffer = nil
		default:
			// Regular character - add to buffer
			if b >= 32 && b < 127 {
				c.inputBuffer = append(c.inputBuffer, b)
			}
		}
	}
}
