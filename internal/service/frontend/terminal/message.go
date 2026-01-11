// Package terminal provides a web-based terminal for admin users.
package terminal

import (
	"encoding/base64"
)

// MessageType represents the type of terminal message.
type MessageType string

const (
	// MessageTypeInput represents user input from the terminal.
	MessageTypeInput MessageType = "input"
	// MessageTypeOutput represents shell output to the terminal.
	MessageTypeOutput MessageType = "output"
	// MessageTypeResize represents a terminal resize event.
	MessageTypeResize MessageType = "resize"
	// MessageTypeClose represents a session close request.
	MessageTypeClose MessageType = "close"
	// MessageTypeError represents an error message.
	MessageTypeError MessageType = "error"
)

// Message represents a terminal message exchanged between client and server.
type Message struct {
	// Type is the message type.
	Type MessageType `json:"type"`
	// Data contains the payload, base64 encoded for binary safety.
	Data string `json:"data,omitempty"`
	// Cols is the terminal width (for resize messages).
	Cols int `json:"cols,omitempty"`
	// Rows is the terminal height (for resize messages).
	Rows int `json:"rows,omitempty"`
}

// NewOutputMessage creates a new output message with base64 encoded data.
func NewOutputMessage(data []byte) *Message {
	return &Message{
		Type: MessageTypeOutput,
		Data: base64.StdEncoding.EncodeToString(data),
	}
}

// NewErrorMessage creates a new error message.
func NewErrorMessage(err string) *Message {
	return &Message{
		Type: MessageTypeError,
		Data: err,
	}
}

// DecodeData decodes the base64 data from the message.
func (m *Message) DecodeData() ([]byte, error) {
	if m.Data == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(m.Data)
}
