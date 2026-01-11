package terminal

import (
	"net/http"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"nhooyr.io/websocket"
)

// Handler handles WebSocket connections for the terminal.
type Handler struct {
	authService  *authservice.Service
	auditService *audit.Service
	shell        string
}

// NewHandler creates a new terminal handler.
func NewHandler(authSvc *authservice.Service, auditSvc *audit.Service, shell string) *Handler {
	return &Handler{
		authService:  authSvc,
		auditService: auditSvc,
		shell:        shell,
	}
}

// ServeHTTP handles WebSocket upgrade and terminal session.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract token from query parameter (WebSocket doesn't support custom headers during handshake)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	// Validate token and get user
	user, err := h.authService.GetUserFromToken(ctx, token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Check admin role
	if !user.Role.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Get client IP address
	ipAddress := getClientIP(r)

	// Upgrade to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow all origins for now. In production, you might want to restrict this.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	// Create and run session
	session := NewSession(user, h.shell, conn, ipAddress)
	_ = session.Run(ctx, h.auditService)
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// GetDefaultShell returns the default shell for the current system.
func GetDefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
