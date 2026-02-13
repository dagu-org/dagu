package terminal

import (
	"net/http"
	"os"

	"github.com/coder/websocket"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	frontendauth "github.com/dagu-org/dagu/internal/service/frontend/auth"
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

// ServeHTTP handles WebSocket upgrade and terminal connection.
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
	ipAddress := frontendauth.GetClientIP(r)

	// Upgrade to WebSocket
	// InsecureSkipVerify allows any origin since access is already protected by token auth
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	// Create and run connection
	tc := NewConnection(user, h.shell, conn, ipAddress)
	_ = tc.Run(ctx, h.auditService)
}

// GetDefaultShell returns the default shell for the current system.
func GetDefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
