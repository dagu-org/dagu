package agent

import "errors"

var (
	// ErrMessageRequired is returned when a chat request has an empty message.
	ErrMessageRequired = errors.New("message is required")
	// ErrAgentNotConfigured is returned when the LLM provider cannot be resolved.
	ErrAgentNotConfigured = errors.New("agent is not configured properly")
	// ErrFailedToProcessMessage is returned when the session manager fails to accept a message.
	ErrFailedToProcessMessage = errors.New("failed to process message")
	// ErrFailedToCancel is returned when a session cancellation fails.
	ErrFailedToCancel = errors.New("failed to cancel session")
	// ErrPromptIDRequired is returned when a user response lacks a prompt_id.
	ErrPromptIDRequired = errors.New("prompt_id is required")
	// ErrPromptExpired is returned when the prompt has expired or was already answered.
	ErrPromptExpired = errors.New("prompt expired or already answered")
)
