package server

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/go-chi/chi/v5"
)

func (svr *Server) defaultRoutes(ctx context.Context, r *chi.Mux) *chi.Mux {
	// Always allow API routes to work
	if svr.headless {
		logger.Info(ctx, "Headless mode enabled: UI is disabled, but API remains active")

		// Only register API routes, skip Web UI routes
		return r
	}

	// Serve assets (optional, remove if not needed)
	r.Get("/assets/*", svr.handleGetAssets())

	// Serve UI pages (disable when headless)
	r.Get("/*", svr.handleRequest(ctx))

	return r
}

func (svr *Server) handleRequest(ctx context.Context) http.HandlerFunc {
	renderFunc := svr.useTemplate(ctx, "index.gohtml", "index")
	return func(w http.ResponseWriter, _ *http.Request) {
		renderFunc(w, nil)
	}
}

func (svr *Server) handleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(svr.assets)).ServeHTTP(w, r)
	}
}
