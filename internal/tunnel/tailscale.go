package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tailscale.com/tsnet"
)

// TailscaleProvider implements the Provider interface for Tailscale.
type TailscaleProvider struct {
	config    *TailscaleConfig
	stateDir  string
	mu        sync.RWMutex
	status    Status
	publicURL string
	startedAt time.Time
	errMsg    string
	server    *tsnet.Server
	listener  net.Listener
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewTailscaleProvider creates a new Tailscale tunnel provider.
func NewTailscaleProvider(cfg *TailscaleConfig, dataDir string) (*TailscaleProvider, error) {
	// Set default hostname
	hostname := cfg.Hostname
	if hostname == "" {
		hostname = "dagu"
	}

	// Set state directory
	stateDir := cfg.StateDir
	if stateDir == "" {
		stateDir = filepath.Join(dataDir, "tailscale")
	}

	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create tailscale state directory: %w", err)
	}

	return &TailscaleProvider{
		config:   cfg,
		stateDir: stateDir,
		status:   StatusDisabled,
	}, nil
}

// Name returns the provider name.
func (p *TailscaleProvider) Name() ProviderType {
	return ProviderTailscale
}

// Start initiates the Tailscale connection.
func (p *TailscaleProvider) Start(ctx context.Context, localAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == StatusConnected || p.status == StatusConnecting {
		return nil
	}

	p.status = StatusConnecting
	p.startedAt = time.Now()
	p.errMsg = ""

	// Create a cancellable context for the tunnel
	tunnelCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})

	// Create tsnet server
	hostname := p.config.Hostname
	if hostname == "" {
		hostname = "dagu"
	}

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      p.stateDir,
		Logf:     func(format string, args ...any) {}, // Silent logging
	}

	// Set auth key if provided
	if p.config.AuthKey != "" {
		srv.AuthKey = p.config.AuthKey
	}

	p.server = srv

	// Start the tunnel in a goroutine
	go p.runTunnel(tunnelCtx, localAddr)

	return nil
}

// runTunnel runs the Tailscale tunnel.
func (p *TailscaleProvider) runTunnel(ctx context.Context, localAddr string) {
	defer close(p.done)

	// Start the tsnet server
	if err := p.server.Start(); err != nil {
		p.setError(fmt.Sprintf("failed to start tailscale: %v", err))
		return
	}

	// Get the local client to check status
	lc, err := p.server.LocalClient()
	if err != nil {
		p.setError(fmt.Sprintf("failed to get tailscale client: %v", err))
		return
	}

	// Wait for the connection to be ready
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		status, err := lc.Status(ctx)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if status.BackendState == "Running" {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Get the DNS name
	status, err := lc.Status(ctx)
	if err != nil {
		p.setError(fmt.Sprintf("failed to get tailscale status: %v", err))
		return
	}

	dnsName := status.Self.DNSName
	if dnsName != "" && dnsName[len(dnsName)-1] == '.' {
		dnsName = dnsName[:len(dnsName)-1]
	}

	p.mu.Lock()
	p.publicURL = fmt.Sprintf("https://%s", dnsName)
	p.status = StatusConnected
	p.mu.Unlock()

	// Create listener based on funnel mode
	var ln net.Listener
	if p.config.Funnel {
		// Use Funnel for public access
		ln, err = p.server.ListenFunnel("tcp", ":443")
		if err != nil {
			p.setError(fmt.Sprintf("failed to start funnel listener: %v", err))
			return
		}
	} else {
		// Use regular TLS listener for tailnet-only access
		ln, err = p.server.ListenTLS("tcp", ":443")
		if err != nil {
			p.setError(fmt.Sprintf("failed to start TLS listener: %v", err))
			return
		}
	}

	p.mu.Lock()
	p.listener = ln
	p.mu.Unlock()

	// Create reverse proxy to local server
	targetURL, err := url.Parse(fmt.Sprintf("http://%s", localAddr))
	if err != nil {
		p.setError(fmt.Sprintf("invalid local address: %v", err))
		ln.Close()
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "Tunnel proxy error: "+err.Error())
	}

	// Start HTTP server on the Tailscale listener
	httpSrv := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		if ctx.Err() == nil {
			p.setError(fmt.Sprintf("tunnel server error: %v", err))
		}
	}
}

// Stop gracefully shuts down the tunnel.
func (p *TailscaleProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	cancel := p.cancel
	server := p.server
	listener := p.listener
	done := p.done
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if listener != nil {
		_ = listener.Close()
	}

	if server != nil {
		_ = server.Close()
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
	p.server = nil
	p.listener = nil
	p.done = nil
	p.mu.Unlock()

	return nil
}

// Info returns current tunnel information.
func (p *TailscaleProvider) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	mode := "direct"
	if p.config.Funnel {
		mode = "funnel"
	}

	return Info{
		Provider:  ProviderTailscale,
		Status:    p.status,
		PublicURL: p.publicURL,
		Error:     p.errMsg,
		StartedAt: p.startedAt,
		Mode:      mode,
		IsPublic:  p.config.Funnel,
	}
}

// PublicURL returns the public URL when connected.
func (p *TailscaleProvider) PublicURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.publicURL
}

// IsPublic returns true if Funnel is enabled.
func (p *TailscaleProvider) IsPublic() bool {
	return p.config.Funnel
}

// setError sets the error status.
func (p *TailscaleProvider) setError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = StatusError
	p.errMsg = msg
}
