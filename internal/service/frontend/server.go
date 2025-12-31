package frontend

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	authmodel "github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/persistence/fileapikey"
	"github.com/dagu-org/dagu/internal/persistence/fileuser"
	"github.com/dagu-org/dagu/internal/persistence/filewebhook"
	"github.com/dagu-org/dagu/internal/runtime"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	apiv1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	apiv2 "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
	"github.com/dagu-org/dagu/internal/service/frontend/auth"
	"github.com/dagu-org/dagu/internal/service/frontend/metrics"
	"github.com/dagu-org/dagu/internal/service/resource"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// Server represents the HTTP server for the frontend application
type Server struct {
	apiV1       *apiv1.API
	apiV2       *apiv2.API
	config      *config.Config
	httpServer  *http.Server
	funcsConfig funcsConfig
}

// NewServer constructs a Server configured from cfg and the provided stores, managers, and services.
// It extracts remote node names from cfg.Server.RemoteNodes, initializes apiV1 and apiV2 with the given dependencies, and populates the Server's funcsConfig fields from cfg.
// NewServer constructs and returns a Server configured from the provided configuration,
// stores, managers, and services.
// It initializes API v1 and v2, populates the server configuration (including UI function
// configuration), and wires an initialized builtin auth service into API v2 when
// cfg.Server.Auth.Mode is set to builtin.
// Returns the constructed *Server, or an error if initialization fails (for example,
// when the configured builtin auth service fails to initialize).
func NewServer(cfg *config.Config, dr execution.DAGStore, drs execution.DAGRunStore, qs execution.QueueStore, ps execution.ProcStore, drm runtime.Manager, cc coordinator.Client, sr execution.ServiceRegistry, mr *prometheus.Registry, collector *telemetry.Collector, rs *resource.Service) (*Server, error) {
	var remoteNodes []string
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	// Build API options
	var apiOpts []apiv2.APIOption

	// Initialize auth service if builtin mode is enabled
	if cfg.Server.Auth.Mode == config.AuthModeBuiltin {
		authSvc, err := initBuiltinAuthService(cfg, collector)
		if err != nil {
			// Fail fast: if auth is configured but fails to initialize, return error
			// to prevent server from running without expected authentication
			return nil, fmt.Errorf("failed to initialize builtin auth service: %w", err)
		}
		apiOpts = append(apiOpts, apiv2.WithAuthService(authSvc))
	}

	return &Server{
		apiV1:  apiv1.New(dr, drs, drm, cfg),
		apiV2:  apiv2.New(dr, drs, qs, ps, drm, cfg, cc, sr, mr, rs, apiOpts...),
		config: cfg,
		funcsConfig: funcsConfig{
			NavbarColor:           cfg.UI.NavbarColor,
			NavbarTitle:           cfg.UI.NavbarTitle,
			BasePath:              cfg.Server.BasePath,
			APIBasePath:           cfg.Server.APIBasePath,
			TZ:                    cfg.Core.TZ,
			TzOffsetInSec:         cfg.Core.TzOffsetInSec,
			MaxDashboardPageLimit: cfg.UI.MaxDashboardPageLimit,
			RemoteNodes:           remoteNodes,
			Permissions:           cfg.Server.Permissions,
			Paths:                 cfg.Paths,
			AuthMode:              cfg.Server.Auth.Mode,
		},
	}, nil
}

// initBuiltinAuthService creates a file-based user store, constructs the builtin
// authentication service, and ensures a default admin user exists.
// If the admin password is auto-generated, the password is printed to stdout.
// It returns the initialized auth service or an error if any step fails.
func initBuiltinAuthService(cfg *config.Config, collector *telemetry.Collector) (*authservice.Service, error) {
	ctx := context.Background()

	// Validate token secret is configured
	if cfg.Server.Auth.Builtin.Token.Secret == "" {
		return nil, fmt.Errorf("builtin auth requires a non-empty token secret (set DAGU_AUTH_TOKEN_SECRET or server.auth.builtin.token.secret)")
	}

	// Create file-based user store
	userStore, err := fileuser.New(cfg.Paths.UsersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create user store: %w", err)
	}

	// Create file-based API key store with cache
	cacheLimits := cfg.Cache.Limits()
	apiKeyCache := fileutil.NewCache[*authmodel.APIKey]("api_key", cacheLimits.APIKey.Limit, cacheLimits.APIKey.TTL)
	apiKeyCache.StartEviction(ctx)
	if collector != nil {
		collector.RegisterCache(apiKeyCache)
	}
	apiKeyStore, err := fileapikey.New(cfg.Paths.APIKeysDir, fileapikey.WithFileCache(apiKeyCache))
	if err != nil {
		return nil, fmt.Errorf("failed to create API key store: %w", err)
	}

	// Create file-based webhook store with cache
	webhookCache := fileutil.NewCache[*authmodel.Webhook]("webhook", cacheLimits.Webhook.Limit, cacheLimits.Webhook.TTL)
	webhookCache.StartEviction(ctx)
	if collector != nil {
		collector.RegisterCache(webhookCache)
	}
	webhookStore, err := filewebhook.New(cfg.Paths.WebhooksDir, filewebhook.WithFileCache(webhookCache))
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook store: %w", err)
	}

	// Create auth service with configuration
	authConfig := authservice.Config{
		TokenSecret: cfg.Server.Auth.Builtin.Token.Secret,
		TokenTTL:    cfg.Server.Auth.Builtin.Token.TTL,
	}
	authSvc := authservice.New(userStore, authConfig,
		authservice.WithAPIKeyStore(apiKeyStore),
		authservice.WithWebhookStore(webhookStore),
	)

	// Ensure admin user exists
	password, created, err := authSvc.EnsureAdminUser(
		ctx,
		cfg.Server.Auth.Builtin.Admin.Username,
		cfg.Server.Auth.Builtin.Admin.Password,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure admin user: %w", err)
	}

	if created {
		if cfg.Server.Auth.Builtin.Admin.Password == "" {
			// Password was auto-generated, print to stdout (not to structured logs)
			// which may be shipped to external systems)
			fmt.Printf("\n"+
				"================================================================================\n"+
				"  ADMIN USER CREATED\n"+
				"  Username: %s\n"+
				"  Password: %s\n"+
				"  NOTE: Please change this password immediately!\n"+
				"================================================================================\n\n",
				cfg.Server.Auth.Builtin.Admin.Username, password)
		} else {
			logger.Info(ctx, "Created admin user",
				slog.String("username", cfg.Server.Auth.Builtin.Admin.Username))
		}
	}

	return authSvc, nil
}

// Serve starts the HTTP server and configures routes
func (srv *Server) Serve(ctx context.Context) error {
	// Setup logger for HTTP requests
	requestLogger := httplog.NewLogger("http", httplog.Options{
		LogLevel:         slog.LevelDebug,
		JSON:             srv.config.Core.LogFormat == "json",
		Concise:          true,
		RequestHeaders:   true,
		MessageFieldName: "msg",
		ResponseHeaders:  true,
	})

	// Create router with middleware
	r := chi.NewMux()
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))
	r.Use(httplog.RequestLogger(requestLogger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Update to specific origins for better security
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Content-Encoding", "Accept"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Use(middleware.RedirectSlashes)

	// Configure API paths
	apiV1BasePath, apiV2BasePath := srv.configureAPIPaths()
	schema := srv.getSchema()

	// Set up routes
	if err := srv.setupRoutes(ctx, r); err != nil {
		return err
	}

	// Configure API routes
	if err := srv.setupAPIRoutes(ctx, r, apiV1BasePath, apiV2BasePath, schema); err != nil {
		return err
	}

	// Configure and start the server
	addr := net.JoinHostPort(srv.config.Server.Host, strconv.Itoa(srv.config.Server.Port))
	srv.httpServer = &http.Server{
		Handler:           r,
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second, // Added IdleTimeout for better resource management
		WriteTimeout:      60 * time.Second,  // Added WriteTimeout for better resource management
	}

	// Start metrics collection
	metrics.StartUptime(ctx)

	// Start the server in a goroutine
	go srv.startServer(ctx, addr)

	// Set up graceful shutdown
	srv.setupGracefulShutdown(ctx)
	return nil
}

// configureAPIPaths returns the properly formatted API paths
func (srv *Server) configureAPIPaths() (string, string) {
	apiV1BasePath := path.Join(srv.config.Server.BasePath, "api/v1")
	if !strings.HasPrefix(apiV1BasePath, "/") {
		apiV1BasePath = "/" + apiV1BasePath
	}

	apiV2BasePath := path.Join(srv.config.Server.BasePath, "api/v2")
	if !strings.HasPrefix(apiV2BasePath, "/") {
		apiV2BasePath = "/" + apiV2BasePath
	}

	return apiV1BasePath, apiV2BasePath
}

// getSchema returns the schema (http or https) based on TLS configuration
func (srv *Server) getSchema() string {
	if srv.config.Server.TLS != nil {
		return "https"
	}
	return "http"
}

// setupRoutes configures the web UI routes
func (srv *Server) setupRoutes(ctx context.Context, r *chi.Mux) error {
	// Skip UI routes in headless mode
	if srv.config.Server.Headless {
		logger.Info(ctx, "Headless mode enabled: UI is disabled, but API remains active")
		return nil
	}

	// Serve assets with proper cache control
	basePath := srv.config.Server.BasePath
	if evaluatedBasePath, err := cmdutil.EvalString(ctx, basePath); err != nil {
		logger.Warn(ctx, "Failed to evaluate server base path",
			tag.Path(basePath),
			tag.Error(err))
	} else {
		basePath = evaluatedBasePath
	}
	assetsPath := path.Join(strings.TrimRight(basePath, "/"), "assets/*")
	if !strings.HasPrefix(assetsPath, "/") {
		assetsPath = "/" + assetsPath
	}

	// Create a file server for the embedded assets
	fileServer := http.FileServer(http.FS(assetsFS))

	// If there's a base path, we need to strip it from the request URL
	if basePath != "" && basePath != "/" {
		fileServer = http.StripPrefix(strings.TrimRight(basePath, "/"), fileServer)
	}

	r.Get(assetsPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		if ctype := mime.TypeByExtension(path.Ext(r.URL.Path)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		fileServer.ServeHTTP(w, r)
	})

	// Serve UI pages
	indexHandler := srv.useTemplate(ctx, "index.gohtml", "index")

	// Initialize OIDC if enabled
	authConfig := srv.config.Server.Auth
	authOIDC := authConfig.OIDC

	var oidcAuthOptions *auth.Options
	if authConfig.OIDC.ClientId != "" && authConfig.OIDC.ClientSecret != "" && authOIDC.Issuer != "" {
		oidcCfg, err := auth.InitVerifierAndConfig(authOIDC)
		if err != nil {
			return fmt.Errorf("failed to initialize OIDC: %w", err)
		}
		oidcAuthOptions = &auth.Options{
			OIDCAuthEnabled: true,
			OIDCWhitelist:   authConfig.OIDC.Whitelist,
			OIDCProvider:    oidcCfg.Provider,
			OIDCVerify:      oidcCfg.Verifier,
			OIDCConfig:      oidcCfg.Config,
		}
	}

	r.Route("/", func(r chi.Router) {
		if oidcAuthOptions != nil {
			r.Use(auth.OIDCMiddleware(*oidcAuthOptions))
		}
		r.Get("/*", func(w http.ResponseWriter, _ *http.Request) {
			indexHandler(w, nil)
		})
	})
	return nil
}

// setupAPIRoutes configures the API routes for both versions
func (srv *Server) setupAPIRoutes(ctx context.Context, r *chi.Mux, apiV1BasePath, apiV2BasePath, schema string) error {
	var setupErr error

	r.Route(apiV1BasePath, func(r chi.Router) {
		if srv.config.Server.Auth.Mode != config.AuthModeNone {
			// v1 API is not available in auth mode - it doesn't support authentication
			logger.Info(ctx, "Authentication enabled: V1 API is disabled, use V2 API instead",
				slog.String("authMode", string(srv.config.Server.Auth.Mode)))
			return
		}
		url := fmt.Sprintf("%s://%s:%d%s", schema, srv.config.Server.Host, srv.config.Server.Port, apiV1BasePath)
		if err := srv.apiV1.ConfigureRoutes(r, url); err != nil {
			logger.Error(ctx, "Failed to configure v1 API routes", tag.Error(err))
			setupErr = err
		}
	})

	r.Route(apiV2BasePath, func(r chi.Router) {
		url := fmt.Sprintf("%s://%s:%d%s", schema, srv.config.Server.Host, srv.config.Server.Port, apiV2BasePath)
		if err := srv.apiV2.ConfigureRoutes(ctx, r, url); err != nil {
			logger.Error(ctx, "Failed to configure v2 API routes", tag.Error(err))
			setupErr = err
		}
	})

	return setupErr
}

// startServer starts the HTTP server with or without TLS
func (srv *Server) startServer(ctx context.Context, addr string) {
	logger.Info(ctx, "Server is starting", tag.Addr(addr))

	var err error
	if srv.config.Server.TLS != nil {
		// Use TLS configuration
		logger.Info(ctx, "Starting TLS server",
			tag.Cert(srv.config.Server.TLS.CertFile),
			slog.String("key", srv.config.Server.TLS.KeyFile))
		err = srv.httpServer.ListenAndServeTLS(srv.config.Server.TLS.CertFile, srv.config.Server.TLS.KeyFile)
	} else {
		// Use standard HTTP
		err = srv.httpServer.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		logger.Error(ctx, "Server failed to start or unexpected shutdown", tag.Error(err))
	}
}

// Shutdown gracefully shuts down the server
func (srv *Server) Shutdown(ctx context.Context) error {
	if srv.httpServer != nil {
		logger.Info(ctx, "Server is shutting down", tag.Addr(srv.httpServer.Addr))

		// Create a context with timeout for shutdown
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		srv.httpServer.SetKeepAlivesEnabled(false)
		return srv.httpServer.Shutdown(shutdownCtx)
	}
	return nil
}

// setupGracefulShutdown configures signal handling for graceful server shutdown
func (srv *Server) setupGracefulShutdown(ctx context.Context) {
	// In the original implementation, this was blocking, which is likely why
	// our modified version exits immediately. Let's make it block again.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block here until context is done or signal is received
	select {
	case <-ctx.Done():
		logger.Info(ctx, "Context done, shutting down server")
	case sig := <-quit:
		logger.Info(ctx, "Received shutdown signal", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.httpServer.SetKeepAlivesEnabled(false)
	if err := srv.httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Failed to shutdown server gracefully", tag.Error(err))
	}
}
