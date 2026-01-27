package tunnel

import (
	"context"
	"time"
)

// Status represents the current state of a tunnel connection.
type Status string

const (
	StatusDisabled     Status = "disabled"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusReconnecting Status = "reconnecting"
	StatusError        Status = "error"
)

// ProviderType represents the tunnel provider.
type ProviderType string

const (
	ProviderTailscale ProviderType = "tailscale"
)

// Info contains information about an active tunnel.
type Info struct {
	Provider  ProviderType `json:"provider"`
	Status    Status       `json:"status"`
	PublicURL string       `json:"publicUrl,omitempty"`
	Error     string       `json:"error,omitempty"`
	StartedAt time.Time    `json:"startedAt,omitempty"`
	Mode      string       `json:"mode"` // "direct" or "funnel"
	IsPublic  bool         `json:"isPublic"`
}

// Provider defines the interface that all tunnel providers must implement.
type Provider interface {
	// Name returns the provider name (e.g., "tailscale").
	Name() ProviderType

	// Start initiates the tunnel connection.
	// The localAddr is the local address to tunnel (e.g., "127.0.0.1:8080").
	Start(ctx context.Context, localAddr string) error

	// Stop gracefully shuts down the tunnel.
	Stop(ctx context.Context) error

	// Info returns current tunnel information.
	Info() Info

	// PublicURL returns the public URL when connected.
	// Returns empty string if not connected.
	PublicURL() string

	// IsPublic returns true if the tunnel exposes the service to the public internet.
	IsPublic() bool
}
