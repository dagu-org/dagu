package tunnel

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CloudflareProvider implements the Provider interface for Cloudflare Tunnels.
// It uses the cloudflared binary as a subprocess for reliable operation.
type CloudflareProvider struct {
	config    *CloudflareConfig
	mu        sync.RWMutex
	status    Status
	publicURL string
	startedAt time.Time
	errMsg    string
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	done      chan struct{}
}

// tunnelToken represents the decoded Cloudflare tunnel token.
type tunnelToken struct {
	AccountTag   string    `json:"a"`
	TunnelSecret []byte    `json:"s"`
	TunnelID     uuid.UUID `json:"t"`
}

// NewCloudflareProvider creates a new Cloudflare tunnel provider.
func NewCloudflareProvider(cfg *CloudflareConfig) (*CloudflareProvider, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("cloudflare tunnel token is required for named tunnels")
	}

	// Validate token format
	if _, err := parseToken(cfg.Token); err != nil {
		return nil, fmt.Errorf("invalid tunnel token: %w", err)
	}

	// Check if cloudflared binary is available
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return nil, fmt.Errorf("cloudflared binary not found in PATH: %w (install from https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)", err)
	}

	return &CloudflareProvider{
		config: cfg,
		status: StatusDisabled,
	}, nil
}

// Name returns the provider name.
func (p *CloudflareProvider) Name() ProviderType {
	return ProviderCloudflare
}

// Start initiates the Cloudflare tunnel connection.
func (p *CloudflareProvider) Start(ctx context.Context, localAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == StatusConnected || p.status == StatusConnecting {
		return nil
	}

	p.status = StatusConnecting
	p.startedAt = time.Now()
	p.errMsg = ""

	// Parse token to get tunnel ID for URL construction
	token, err := parseToken(p.config.Token)
	if err != nil {
		p.status = StatusError
		p.errMsg = fmt.Sprintf("invalid tunnel token: %v", err)
		return fmt.Errorf("failed to parse tunnel token: %w", err)
	}

	// Create a cancellable context for the tunnel
	tunnelCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})

	// Build the public URL
	hostname := p.config.Hostname
	if hostname == "" {
		hostname = fmt.Sprintf("%s.cfargotunnel.com", token.TunnelID.String())
	}
	p.publicURL = fmt.Sprintf("https://%s", hostname)

	// Start the tunnel in a goroutine
	go p.runTunnel(tunnelCtx, localAddr)

	return nil
}

// runTunnel runs the cloudflared tunnel as a subprocess.
func (p *CloudflareProvider) runTunnel(ctx context.Context, localAddr string) {
	defer close(p.done)

	// Build cloudflared command
	// cloudflared tunnel --url http://localhost:8080 run --token <TOKEN>
	args := []string{
		"tunnel",
		"--url", fmt.Sprintf("http://%s", localAddr),
		"run",
		"--token", p.config.Token,
	}

	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	cmd.Env = os.Environ()

	// Capture stderr for status updates
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.setError(fmt.Sprintf("failed to create stderr pipe: %v", err))
		return
	}

	p.mu.Lock()
	p.cmd = cmd
	p.mu.Unlock()

	// Start the process
	if err := cmd.Start(); err != nil {
		p.setError(fmt.Sprintf("failed to start cloudflared: %v", err))
		return
	}

	// Monitor stderr for connection status
	go p.monitorOutput(stderr)

	// Wait for the process to exit
	err = cmd.Wait()
	if ctx.Err() != nil {
		// Context was cancelled, this is expected
		return
	}
	if err != nil {
		p.setError(fmt.Sprintf("cloudflared exited with error: %v", err))
	}
}

// monitorOutput monitors cloudflared stderr for connection status.
func (p *CloudflareProvider) monitorOutput(stderr interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(stderr)
	connectedPattern := regexp.MustCompile(`Connection .* registered`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for successful connection
		if connectedPattern.MatchString(line) {
			p.mu.Lock()
			if p.status == StatusConnecting {
				p.status = StatusConnected
			}
			p.mu.Unlock()
		}
	}
}

// Stop gracefully shuts down the tunnel.
func (p *CloudflareProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	cancel := p.cancel
	cmd := p.cmd
	done := p.done
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Wait for process to exit
	if cmd != nil && cmd.Process != nil {
		// Send SIGTERM first
		_ = cmd.Process.Signal(os.Interrupt)

		// Wait with timeout
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			// Force kill if still running
			_ = cmd.Process.Kill()
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		}
	}

	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	p.mu.Lock()
	p.status = StatusDisabled
	p.publicURL = ""
	p.cancel = nil
	p.cmd = nil
	p.done = nil
	p.mu.Unlock()

	return nil
}

// Info returns current tunnel information.
func (p *CloudflareProvider) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return Info{
		Provider:  ProviderCloudflare,
		Status:    p.status,
		PublicURL: p.publicURL,
		Error:     p.errMsg,
		StartedAt: p.startedAt,
		Mode:      "named",
		IsPublic:  true,
	}
}

// PublicURL returns the public URL when connected.
func (p *CloudflareProvider) PublicURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.publicURL
}

// IsPublic returns true - Cloudflare tunnels are always public.
func (p *CloudflareProvider) IsPublic() bool {
	return true
}

// setError sets the error status.
func (p *CloudflareProvider) setError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = StatusError
	p.errMsg = msg
}

// parseToken decodes a Cloudflare tunnel token.
func parseToken(tokenStr string) (*tunnelToken, error) {
	data, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	var token tunnelToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if token.AccountTag == "" || token.TunnelID == uuid.Nil || len(token.TunnelSecret) == 0 {
		return nil, fmt.Errorf("invalid token: missing required fields")
	}

	return &token, nil
}
