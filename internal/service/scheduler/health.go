// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HealthServer represents the health check HTTP server for the scheduler
type HealthServer struct {
	mu         sync.Mutex
	server     *http.Server
	listener   net.Listener
	port       int
	listenAddr string
	boundAddr  string
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// NewHealthServer creates a new health check server
func NewHealthServer(port int) *HealthServer {
	return &HealthServer{
		port:       port,
		listenAddr: fmt.Sprintf(":%d", port),
	}
}

func newHealthServerWithAddr(addr string) *HealthServer {
	return &HealthServer{
		listenAddr: addr,
	}
}

// URL returns the currently bound HTTP base URL for the health server.
// It is primarily intended for tests and diagnostics.
func (h *HealthServer) URL() string {
	if h == nil {
		return ""
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.boundAddr == "" {
		return ""
	}
	return "http://" + h.boundAddr
}

// Start starts the health check server
func (h *HealthServer) Start(ctx context.Context) error {
	if h.listenAddr == "" || (h.port == 0 && h.listenAddr == ":0") {
		logger.Info(ctx, "Scheduler health check server disabled (port=0)")
		return nil // Health check server disabled
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	router.Get("/health", h.healthHandler)

	server := &http.Server{
		Addr:              h.listenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", h.listenAddr)
	if err != nil {
		return err
	}
	boundAddr := listener.Addr().String()

	h.mu.Lock()
	if h.server != nil {
		h.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	h.server = server
	h.listener = listener
	h.boundAddr = boundAddr
	h.mu.Unlock()

	// Start server in a goroutine
	go func(server *http.Server, listener net.Listener) {
		logger.Info(ctx, "Starting scheduler health check server", tag.Port(h.port))
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Health check server error", tag.Error(err))
		}
	}(server, listener)

	return nil
}

// Stop gracefully stops the health check server
func (h *HealthServer) Stop(ctx context.Context) error {
	h.mu.Lock()
	server := h.server
	listener := h.listener
	if server == nil {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	logger.Info(ctx, "Stopping scheduler health check server")

	// Give the server 5 seconds to shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if listener != nil {
		_ = listener.Close()
	}

	h.mu.Lock()
	if h.server == server {
		h.server = nil
		h.listener = nil
		h.boundAddr = ""
	}
	h.mu.Unlock()

	if err != nil {
		logger.Error(ctx, "Failed to shutdown scheduler health check server", tag.Error(err))
		return err
	}
	return nil
}

// healthHandler handles the /health endpoint
func (h *HealthServer) healthHandler(w http.ResponseWriter, _ *http.Request) {
	response := HealthResponse{
		Status: "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but don't write anything else to avoid corrupting response
		logger.Error(context.Background(), "Failed to encode health response", tag.Error(err))
	}
}
