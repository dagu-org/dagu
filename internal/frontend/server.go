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

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/api/v1"
	"github.com/dagu-org/dagu/internal/frontend/metrics"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
)

type Server struct {
	api        *api.API
	config     *config.Config
	httpServer *http.Server
}

func NewServer(
	api *api.API,
	cfg *config.Config,
) *Server {
	return &Server{
		api:    api,
		config: cfg,
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
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	basePath := path.Join(srv.config.Server.BasePath, "api/v1")
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	schema := "http"
	if srv.config.Server.TLS != nil {
		schema = "https"
	}
	url := fmt.Sprintf("%s://%s%s", schema, srv.config.Server.Host, basePath)

	r.Route(basePath, func(r chi.Router) {
		if err := srv.api.ConfigureRoutes(r, url); err != nil {
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
		if err := srv.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Failed to start server", "err", err)
		}
	}()

	srv.gracefulShutdown(ctx)
	return nil
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
