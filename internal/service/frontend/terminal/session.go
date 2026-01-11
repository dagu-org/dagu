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

// Session represents an interactive terminal session.
type Session struct {
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

// NewSession creates a new terminal session.
func NewSession(user *auth.User, shell string, conn *websocket.Conn, ipAddress string) *Session {
	return &Session{
		ID:         uuid.New().String(),
		User:       user,
		IPAddress:  ipAddress,
		Shell:      shell,
		Conn:       conn,
		StartTime:  time.Now(),
		LastActive: time.Now(),
	}
}

// Run starts the terminal session and handles communication between
// the WebSocket and the PTY.
func (s *Session) Run(ctx context.Context, auditSvc *audit.Service) error {
	// Log session start
	if auditSvc != nil {
		_ = auditSvc.LogTerminalSessionStart(ctx, s.User.ID, s.User.Username, s.ID, s.IPAddress)
	}

	// Start PTY with shell
	cmd := exec.Command(s.Shell) //nolint:gosec // shell path is from config, not user input
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		s.sendError("Failed to start shell: " + err.Error())
		if auditSvc != nil {
			_ = auditSvc.LogTerminalSessionEnd(ctx, s.User.ID, s.User.Username, s.ID, "pty_error", s.IPAddress)
		}
		return err
	}

	s.ptmx = ptmx
	s.cmd = cmd

	// Set initial size
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	// Create done channel for cleanup
	done := make(chan struct{})
	defer close(done)

	// Read from PTY and write to WebSocket
	go s.readFromPTY(ctx, done)

	// Read from WebSocket and write to PTY
	s.readFromWebSocket(ctx, auditSvc)

	// Wait for command to finish
	_ = cmd.Wait()

	// Log session end
	if auditSvc != nil {
		_ = auditSvc.LogTerminalSessionEnd(ctx, s.User.ID, s.User.Username, s.ID, "closed", s.IPAddress)
	}

	return nil
}

// readFromPTY reads output from the PTY and sends it to the WebSocket.
func (s *Session) readFromPTY(ctx context.Context, done chan struct{}) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.sendError("Shell closed: " + err.Error())
			}
			return
		}

		if n > 0 {
			msg := NewOutputMessage(buf[:n])
			s.sendMessage(ctx, msg)
			s.updateLastActive()
		}
	}
}

// readFromWebSocket reads input from the WebSocket and writes to the PTY.
func (s *Session) readFromWebSocket(ctx context.Context, auditSvc *audit.Service) {
	for {
		_, data, err := s.Conn.Read(ctx)
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		s.updateLastActive()

		switch msg.Type {
		case MessageTypeInput:
			input, err := msg.DecodeData()
			if err != nil {
				continue
			}
			if len(input) > 0 {
				// Write input to PTY
				_, _ = s.ptmx.Write(input)

				// Accumulate input for command logging
				if auditSvc != nil {
					s.accumulateAndLogCommand(ctx, auditSvc, input)
				}
			}

		case MessageTypeResize:
			if msg.Cols > 0 && msg.Cols <= 500 && msg.Rows > 0 && msg.Rows <= 500 {
				_ = pty.Setsize(s.ptmx, &pty.Winsize{
					Rows: uint16(msg.Rows), //nolint:gosec // bounds checked above
					Cols: uint16(msg.Cols), //nolint:gosec // bounds checked above
				})
			}

		case MessageTypeClose:
			s.Close()
			return

		case MessageTypeOutput, MessageTypeError:
			// These are server-to-client message types, ignore if received from client
		}
	}
}

// sendMessage sends a message to the WebSocket.
func (s *Session) sendMessage(ctx context.Context, msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_ = s.Conn.Write(ctx, websocket.MessageText, data)
}

// sendError sends an error message to the WebSocket.
func (s *Session) sendError(errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.sendMessage(ctx, NewErrorMessage(errMsg))
}

// updateLastActive updates the last active timestamp.
func (s *Session) updateLastActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActive = time.Now()
}

// Close closes the terminal session.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.Conn.Close(websocket.StatusNormalClosure, "session closed")
}

// accumulateAndLogCommand accumulates input and logs complete commands when Enter is pressed.
func (s *Session) accumulateAndLogCommand(ctx context.Context, auditSvc *audit.Service, input []byte) {
	for _, b := range input {
		// Handle ANSI escape sequences (e.g., arrow keys send ESC[A, ESC[B, etc.)
		if s.inEscSeq {
			// Escape sequences end with a letter (A-Z, a-z)
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				s.inEscSeq = false
			}
			continue
		}

		switch b {
		case 27: // ESC - start of escape sequence
			s.inEscSeq = true
		case '\r', '\n':
			// Enter pressed - log the accumulated command
			if len(s.inputBuffer) > 0 {
				command := string(s.inputBuffer)
				_ = auditSvc.LogTerminalCommand(ctx, s.User.ID, s.User.Username, s.ID, command, s.IPAddress)
				s.inputBuffer = nil
			}
		case 127, '\b':
			// Backspace - remove last character from buffer
			if len(s.inputBuffer) > 0 {
				s.inputBuffer = s.inputBuffer[:len(s.inputBuffer)-1]
			}
		case 3:
			// Ctrl+C - clear the buffer
			s.inputBuffer = nil
		case 21:
			// Ctrl+U - clear the line
			s.inputBuffer = nil
		default:
			// Regular character - add to buffer
			if b >= 32 && b < 127 {
				s.inputBuffer = append(s.inputBuffer, b)
			}
		}
	}
}
