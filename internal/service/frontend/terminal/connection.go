package terminal

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/service/audit"
	"github.com/google/uuid"
)

// Connection represents an interactive terminal connection.
type Connection struct {
	ID         string
	User       *auth.User
	IPAddress  string
	Shell      string
	Conn       *websocket.Conn
	StartTime  time.Time
	LastActive time.Time

	ptmx        *os.File
	cmd         *exec.Cmd
	mu          sync.Mutex
	closed      bool
	inputBuffer []byte // accumulates input until newline for command logging
	inEscSeq    bool   // true when processing an ANSI escape sequence
}

// NewConnection creates a new terminal connection.
func NewConnection(user *auth.User, shell string, conn *websocket.Conn, ipAddress string) *Connection {
	return &Connection{
		ID:         uuid.New().String(),
		User:       user,
		IPAddress:  ipAddress,
		Shell:      shell,
		Conn:       conn,
		StartTime:  time.Now(),
		LastActive: time.Now(),
	}
}

// Run starts the terminal connection and handles communication between
// the WebSocket and the PTY.
func (c *Connection) Run(ctx context.Context, auditSvc *audit.Service) error {
	// Log connection start
	if auditSvc != nil {
		_ = auditSvc.LogTerminalConnectionStart(ctx, c.User.ID, c.User.Username, c.ID, c.IPAddress)
	}

	// Start PTY with shell
	cmd := exec.Command(c.Shell) //nolint:gosec // shell path is from config, not user input
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		c.sendError("Failed to start shell: " + err.Error())
		if auditSvc != nil {
			_ = auditSvc.LogTerminalConnectionEnd(ctx, c.User.ID, c.User.Username, c.ID, "pty_error", c.IPAddress)
		}
		return err
	}

	c.ptmx = ptmx
	c.cmd = cmd

	// Set initial size
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	// Create done channel for cleanup
	done := make(chan struct{})

	// Use WaitGroup to track goroutine lifecycle
	var wg sync.WaitGroup

	// Read from PTY and write to WebSocket
	wg.Go(func() {
		c.readFromPTY(ctx, done)
	})

	// Read from WebSocket and write to PTY
	c.readFromWebSocket(ctx, auditSvc)

	// Set read deadline to unblock PTY read, then signal done
	_ = ptmx.SetReadDeadline(time.Now())
	close(done)

	// Wait for goroutine to finish
	wg.Wait()

	// Wait for command to finish
	_ = cmd.Wait()

	// Log connection end
	if auditSvc != nil {
		_ = auditSvc.LogTerminalConnectionEnd(ctx, c.User.ID, c.User.Username, c.ID, "closed", c.IPAddress)
	}

	return nil
}

// readFromPTY reads output from the PTY and sends it to the WebSocket.
func (c *Connection) readFromPTY(ctx context.Context, done chan struct{}) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		default:
		}

		n, err := c.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				c.sendError("Shell closed: " + err.Error())
			}
			return
		}

		if n > 0 {
			msg := NewOutputMessage(buf[:n])
			c.sendMessage(ctx, msg)
			c.updateLastActive()
		}
	}
}

// readFromWebSocket reads input from the WebSocket and writes to the PTY.
func (c *Connection) readFromWebSocket(ctx context.Context, auditSvc *audit.Service) {
	for {
		_, data, err := c.Conn.Read(ctx)
		if err != nil {
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
			if len(input) > 0 {
				// Write input to PTY
				_, _ = c.ptmx.Write(input)

				// Accumulate input for command logging
				if auditSvc != nil {
					c.accumulateAndLogCommand(ctx, auditSvc, input)
				}
			}

		case MessageTypeResize:
			if msg.Cols > 0 && msg.Cols <= 500 && msg.Rows > 0 && msg.Rows <= 500 {
				_ = pty.Setsize(c.ptmx, &pty.Winsize{
					Rows: uint16(msg.Rows), //nolint:gosec // bounds checked above
					Cols: uint16(msg.Cols), //nolint:gosec // bounds checked above
				})
			}

		case MessageTypeClose:
			c.Close()
			return

		case MessageTypeOutput, MessageTypeError:
			// These are server-to-client message types, ignore if received from client
		}
	}
}

// sendMessage sends a message to the WebSocket.
func (c *Connection) sendMessage(ctx context.Context, msg *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_ = c.Conn.Write(ctx, websocket.MessageText, data)
}

// sendError sends an error message to the WebSocket.
func (c *Connection) sendError(errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.sendMessage(ctx, NewErrorMessage(errMsg))
}

// updateLastActive updates the last active timestamp.
func (c *Connection) updateLastActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastActive = time.Now()
}

// Close closes the terminal connection.
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	if c.ptmx != nil {
		_ = c.ptmx.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.Conn.Close(websocket.StatusNormalClosure, "connection closed")
}

// accumulateAndLogCommand accumulates input and logs complete commands when Enter is pressed.
func (c *Connection) accumulateAndLogCommand(ctx context.Context, auditSvc *audit.Service, input []byte) {
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
				command := string(c.inputBuffer)
				_ = auditSvc.LogTerminalCommand(ctx, c.User.ID, c.User.Username, c.ID, command, c.IPAddress)
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
