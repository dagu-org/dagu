package frontend

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	apiv1 "github.com/dagu-org/dagu/internal/frontend/api/v1"
	apiv2 "github.com/dagu-org/dagu/internal/frontend/api/v2"
	"github.com/dagu-org/dagu/internal/frontend/metrics"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
)

type Server struct {
	apiV1       *apiv1.API
	apiV2       *apiv2.API
	config      *config.Config
	httpServer  *http.Server
	funcsConfig funcsConfig
}

func NewServer(cfg *config.Config, cli client.Client) *Server {
	var remoteNodes []string
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}
	return &Server{
		apiV1:  apiv1.New(cli, cfg),
		apiV2:  apiv2.New(cli, cfg),
		config: cfg,
		funcsConfig: funcsConfig{
			NavbarColor:           cfg.UI.NavbarColor,
			NavbarTitle:           cfg.UI.NavbarTitle,
			BasePath:              cfg.Server.BasePath,
			APIBasePath:           cfg.Server.APIBasePath,
			TZ:                    cfg.Global.TZ,
			MaxDashboardPageLimit: cfg.UI.MaxDashboardPageLimit,
			RemoteNodes:           remoteNodes,
		},
	}
}

func (srv *Server) Serve(ctx context.Context) error {
	requestLogger := httplog.NewLogger("http", httplog.Options{
		LogLevel:         slog.LevelDebug,
		JSON:             srv.config.Global.LogFormat == "json",
		Concise:          true,
		RequestHeaders:   true,
		MessageFieldName: "msg",
		ResponseHeaders:  true,
	})

	r := chi.NewMux()
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))
	r.Use(httplog.RequestLogger(requestLogger))
	r.Use(withRecoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Update to specific origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	srv.routes(ctx, r)

	apiV1BasePath := path.Join(srv.config.Server.BasePath, "api/v1")
	if !strings.HasPrefix(apiV1BasePath, "/") {
		apiV1BasePath = "/" + apiV1BasePath
	}

	apiV2BasePath := path.Join(srv.config.Server.BasePath, "api/v2")
	if !strings.HasPrefix(apiV2BasePath, "/") {
		apiV2BasePath = "/" + apiV2BasePath
	}

	schema := "http"
	if srv.config.Server.TLS != nil {
		schema = "https"
	}

	r.Route(apiV1BasePath, func(r chi.Router) {
		url := fmt.Sprintf("%s://%s:%d%s", schema, srv.config.Server.Host, srv.config.Server.Port, apiV1BasePath)
		if err := srv.apiV1.ConfigureRoutes(r, url); err != nil {
			logger.Error(ctx, "Failed to configure routes", "err", err)
		}
	})

	r.Route(apiV2BasePath, func(r chi.Router) {
		url := fmt.Sprintf("%s://%s:%d%s", schema, srv.config.Server.Host, srv.config.Server.Port, apiV2BasePath)
		if err := srv.apiV2.ConfigureRoutes(r, url); err != nil {
			logger.Error(ctx, "Failed to configure routes", "err", err)
		}
	})

	addr := net.JoinHostPort(srv.config.Server.Host, strconv.Itoa(srv.config.Server.Port))
	srv.httpServer = &http.Server{
		Handler:           r,
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	metrics.StartUptime(ctx)

	go func() {
		logger.Info(ctx, "Server is starting", "addr", addr)

		var err error
		if srv.config.Server.TLS != nil {
			// Use TLS configuration
			logger.Info(ctx, "Starting TLS server", "cert", srv.config.Server.TLS.CertFile, "key", srv.config.Server.TLS.KeyFile)
			err = srv.httpServer.ListenAndServeTLS(srv.config.Server.TLS.CertFile, srv.config.Server.TLS.KeyFile)
		} else {
			// Use standard HTTP
			err = srv.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Failed to start server", "err", err)
		}
	}()

	srv.gracefulShutdown(ctx)
	return nil
}

func (srv *Server) routes(ctx context.Context, r *chi.Mux) {
	// Always allow API routes to work
	if srv.config.Server.Headless {
		logger.Info(ctx, "Headless mode enabled: UI is disabled, but API remains active")

		// Only register API routes, skip Web UI routes
		return
	}

	// Serve assets (optional, remove if not needed)
	r.Get("/assets/*", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(assetsFS)).ServeHTTP(w, r)
	})

	// Serve UI pages (disable when headless)
	r.Get("/*", func(w http.ResponseWriter, _ *http.Request) {
		srv.useTemplate(ctx, "index.gohtml", "index")(w, nil)
	})
}

func (srv *Server) Shutdown(ctx context.Context) error {
	if srv.httpServer != nil {
		logger.Info(ctx, "Server is shutting down", "addr", srv.httpServer.Addr)
		return srv.httpServer.Shutdown(ctx)
	}
	return nil
}

func (srv *Server) gracefulShutdown(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		logger.Info(ctx, "Context done, shutting down server")
	case <-quit:
		logger.Info(ctx, "Received shutdown signal")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	srv.httpServer.SetKeepAlivesEnabled(false)
	if err := srv.httpServer.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Failed to shutdown server", "err", err)
	}

	logger.Info(ctx, "Server shutdown complete")
}

// This function is adapted from the `recoverer` middleware from the `chi` package.
func withRecoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					// we don't recover http.ErrAbortHandler so the response
					// to the client is aborted, this should not be logged
					panic(rvr)
				}

				st := string(debug.Stack())
				logger.Error(r.Context(), "Panic occurred", "err", rvr, "st", st)

				if r.Header.Get("Connection") != "Upgrade" {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
