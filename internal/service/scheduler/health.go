package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HealthServer represents the health check HTTP server for the scheduler
type HealthServer struct {
	server *http.Server
	port   int
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// NewHealthServer creates a new health check server
func NewHealthServer(port int) *HealthServer {
	return &HealthServer{
		port: port,
	}
}

// Start starts the health check server
func (h *HealthServer) Start(ctx context.Context) error {
	if h.port == 0 {
		logger.Info(ctx, "Scheduler health check server disabled (port=0)")
		return nil // Health check server disabled
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	router.Get("/health", h.healthHandler)

	h.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", h.port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info(ctx, "Starting scheduler health check server", tag.Port(h.port))
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Health check server error", tag.Error(err))
		}
	}()

	return nil
}

// Stop gracefully stops the health check server
func (h *HealthServer) Stop(ctx context.Context) error {
	if h.server == nil {
		return nil
	}

	logger.Info(ctx, "Stopping scheduler health check server")

	// Give the server 5 seconds to shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(shutdownCtx); err != nil {
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
