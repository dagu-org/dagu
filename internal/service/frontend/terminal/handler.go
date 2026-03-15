// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
	manager      *Manager
	shell        string
}

// NewHandler creates a new terminal handler.
func NewHandler(authSvc *authservice.Service, auditSvc *audit.Service, manager *Manager, shell string) *Handler {
	return &Handler{
		authService:  authSvc,
		auditService: auditSvc,
		manager:      manager,
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

	if h.manager != nil {
		if err := h.manager.Acquire(); err != nil {
			status := http.StatusTooManyRequests
			message := "Terminal session limit reached. Please close another terminal and try again."
			if err == ErrManagerShuttingDown {
				status = http.StatusServiceUnavailable
				message = "Terminal is shutting down. Please try again after the server restarts."
			}
			http.Error(w, message, status)
			return
		}
	}

	releasePending := true
	defer func() {
		if releasePending && h.manager != nil {
			h.manager.ReleasePending()
		}
	}()

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
	if h.manager != nil {
		if err := h.manager.Register(tc); err != nil {
			releasePending = false
			h.manager.ReleasePending()
			_ = conn.Close(websocket.StatusTryAgainLater, "terminal unavailable")
			return
		}
		defer h.manager.ReleaseSession(tc.ID)
		releasePending = false
	}

	runCtx := ctx
	if h.manager != nil {
		runCtx = h.manager.Context()
	}
	_ = tc.Run(runCtx, h.auditService)
}

// GetDefaultShell returns the default shell for the current system.
func GetDefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
