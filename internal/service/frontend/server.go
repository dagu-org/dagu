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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	authmodel "github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/telemetry"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/gitsync"
	"github.com/dagu-org/dagu/internal/persis/fileapikey"
	"github.com/dagu-org/dagu/internal/persis/fileaudit"
	"github.com/dagu-org/dagu/internal/persis/fileuser"
	"github.com/dagu-org/dagu/internal/persis/filewebhook"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	apiv1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	apiv2 "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
	"github.com/dagu-org/dagu/internal/service/frontend/auth"
	"github.com/dagu-org/dagu/internal/service/frontend/metrics"
	"github.com/dagu-org/dagu/internal/service/frontend/terminal"
	"github.com/dagu-org/dagu/internal/service/oidcprovision"
	"github.com/dagu-org/dagu/internal/service/resource"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// Server represents the HTTP server for the frontend application
type Server struct {
	apiV1          *apiv1.API
	apiV2          *apiv2.API
	config         *config.Config
	httpServer     *http.Server
	funcsConfig    funcsConfig
	builtinOIDCCfg *auth.BuiltinOIDCConfig // OIDC config for builtin auth mode
	authService    *authservice.Service
	auditService   *audit.Service
	listener       net.Listener // Optional pre-bound listener (for tests)
}

// ServerOption is a functional option for configuring the Server
type ServerOption func(*Server)

// WithListener sets a pre-bound listener for the server.
// When set, the server will use this listener instead of creating its own.
// This is useful for tests to avoid race conditions with port allocation.
func WithListener(l net.Listener) ServerOption {
	return func(s *Server) {
		s.listener = l
	}
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
// The context is used for OIDC provider initialization and should be cancellable
// to allow graceful shutdown during startup.
func NewServer(ctx context.Context, cfg *config.Config, dr exec.DAGStore, drs exec.DAGRunStore, qs exec.QueueStore, ps exec.ProcStore, drm runtime.Manager, cc coordinator.Client, sr exec.ServiceRegistry, mr *prometheus.Registry, collector *telemetry.Collector, rs *resource.Service, opts ...ServerOption) (*Server, error) {
	// Defensive nil-context guard to prevent surprising crashes from older call sites
	if ctx == nil {
		ctx = context.Background()
	}

	var remoteNodes []string
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	// Build API options
	var apiOpts []apiv2.APIOption

	// OIDC config for builtin mode
	var builtinOIDCCfg *auth.BuiltinOIDCConfig
	var oidcEnabled bool
	var oidcButtonLabel string

	// Initialize audit service
	auditSvc, err := initAuditService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audit service: %w", err)
	}
	if auditSvc != nil {
		apiOpts = append(apiOpts, apiv2.WithAuditService(auditSvc))
	}

	// Initialize git sync service if enabled
	syncSvc := initSyncService(ctx, cfg)
	if syncSvc != nil {
		apiOpts = append(apiOpts, apiv2.WithSyncService(syncSvc))
	}

	// Initialize auth service if builtin mode is enabled
	var authSvc *authservice.Service
	if cfg.Server.Auth.Mode == config.AuthModeBuiltin {
		result, err := initBuiltinAuthService(cfg, collector)
		if err != nil {
			// Fail fast: if auth is configured but fails to initialize, return error
			// to prevent server from running without expected authentication
			return nil, fmt.Errorf("failed to initialize builtin auth service: %w", err)
		}
		authSvc = result.AuthService
		apiOpts = append(apiOpts, apiv2.WithAuthService(result.AuthService))

		// Initialize OIDC if configured under builtin mode
		oidcCfg := cfg.Server.Auth.OIDC
		if oidcCfg.IsConfigured() {
			oidcEnabled = true
			oidcButtonLabel = oidcCfg.ButtonLabel

			// Create OIDC provisioning service
			provisionCfg := oidcprovision.Config{
				Issuer:         oidcCfg.Issuer,
				AutoSignup:     oidcCfg.AutoSignup,
				DefaultRole:    authmodel.Role(oidcCfg.RoleMapping.DefaultRole),
				AllowedDomains: oidcCfg.AllowedDomains,
				Whitelist:      oidcCfg.Whitelist,
				RoleMapping: oidcprovision.RoleMapperConfig{
					GroupsClaim:         oidcCfg.RoleMapping.GroupsClaim,
					GroupMappings:       oidcCfg.RoleMapping.GroupMappings,
					RoleAttributePath:   oidcCfg.RoleMapping.RoleAttributePath,
					RoleAttributeStrict: oidcCfg.RoleMapping.RoleAttributeStrict,
					SkipOrgRoleSync:     oidcCfg.RoleMapping.SkipOrgRoleSync,
					DefaultRole:         authmodel.Role(oidcCfg.RoleMapping.DefaultRole),
				},
			}
			provisionSvc, err := oidcprovision.New(result.UserStore, provisionCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create OIDC provisioning service: %w", err)
			}

			// Initialize OIDC provider
			builtinOIDCCfg, err = auth.InitBuiltinOIDCConfig(
				ctx,
				oidcCfg,
				result.AuthService,
				provisionSvc,
				cfg.Server.BasePath,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize builtin OIDC: %w", err)
			}

			logger.Info(ctx, "OIDC enabled for builtin auth mode",
				slog.String("issuer", oidcCfg.Issuer),
				slog.Bool("autoSignup", oidcCfg.AutoSignup),
				slog.String("defaultRole", oidcCfg.RoleMapping.DefaultRole))
		}
	}

	srv := &Server{
		apiV1:          apiv1.New(dr, drs, drm, cfg),
		apiV2:          apiv2.New(dr, drs, qs, ps, drm, cfg, cc, sr, mr, rs, apiOpts...),
		config:         cfg,
		builtinOIDCCfg: builtinOIDCCfg,
		authService:    authSvc,
		auditService:   auditSvc,
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
			OIDCEnabled:           oidcEnabled,
			OIDCButtonLabel:       oidcButtonLabel,
			TerminalEnabled:       cfg.Server.Terminal.Enabled && authSvc != nil,
			GitSyncEnabled:        cfg.GitSync.Enabled,
		},
	}

	// Apply server options
	for _, opt := range opts {
		opt(srv)
	}

	return srv, nil
}

// builtinAuthResult holds the result of initializing builtin auth.
type builtinAuthResult struct {
	AuthService *authservice.Service
	UserStore   authmodel.UserStore
}

// initBuiltinAuthService creates a file-based user store, constructs the builtin
// authentication service, and ensures a default admin user exists.
// If the admin password is auto-generated, the password is printed to stdout.
// It returns the initialized auth service and user store, or an error if any step fails.
func initBuiltinAuthService(cfg *config.Config, collector *telemetry.Collector) (*builtinAuthResult, error) {
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

	return &builtinAuthResult{
		AuthService: authSvc,
		UserStore:   userStore,
	}, nil
}

// initAuditService creates a file-based audit store and service.
// Returns nil service if audit logging is disabled or cannot be configured.
func initAuditService(cfg *config.Config) (*audit.Service, error) {
	// Check if audit logging is enabled
	if !cfg.Server.Audit.Enabled {
		return nil, nil
	}

	// Use AdminLogsDir for audit logs
	auditDir := filepath.Join(cfg.Paths.AdminLogsDir, "audit")

	store, err := fileaudit.New(auditDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit store: %w", err)
	}

	return audit.New(store), nil
}

// initSyncService creates and returns a Git sync service if enabled.
// Returns nil if Git sync is not enabled or not configured.
func initSyncService(ctx context.Context, cfg *config.Config) gitsync.Service {
	gitSyncCfg := cfg.GitSync
	if !gitSyncCfg.Enabled {
		return nil
	}

	syncCfg := gitsync.NewConfigFromGlobal(gitSyncCfg)

	svc := gitsync.NewService(syncCfg, cfg.Paths.DAGsDir, cfg.Paths.DataDir)

	// Start auto-sync if enabled
	if syncCfg.AutoSync.Enabled {
		go func() {
			if err := svc.Start(ctx); err != nil {
				logger.Error(ctx, "Failed to start git sync auto-sync", tag.Error(err))
			}
		}()
		logger.Info(ctx, "Git sync auto-sync started",
			slog.String("repository", syncCfg.Repository),
			slog.String("branch", syncCfg.Branch),
			slog.Int("interval", syncCfg.AutoSync.Interval))
	}

	logger.Info(ctx, "Git sync service initialized",
		slog.String("repository", syncCfg.Repository),
		slog.String("branch", syncCfg.Branch))

	return svc
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

	// Configure terminal WebSocket route (admin-only, requires builtin auth)
	if srv.config.Server.Terminal.Enabled && srv.authService != nil {
		srv.setupTerminalRoute(ctx, r, apiV2BasePath)
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

	// Log before starting goroutine to avoid race condition in tests
	logger.Info(ctx, "Server is starting", tag.Addr(addr))

	// Start the server in a goroutine
	go srv.startServer(ctx)

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

	// Initialize OIDC if enabled (legacy mode: oidc)
	authConfig := srv.config.Server.Auth
	authOIDC := authConfig.OIDC

	var oidcAuthOptions *auth.Options
	if authConfig.Mode == config.AuthModeOIDC && authConfig.OIDC.ClientId != "" && authConfig.OIDC.ClientSecret != "" && authOIDC.Issuer != "" {
		oidcCfg, err := auth.InitVerifierAndConfig(ctx, authOIDC)
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

	// Add OIDC routes for builtin auth mode (if configured)
	if srv.builtinOIDCCfg != nil {
		r.Get("/oidc-login", auth.BuiltinOIDCLoginHandler(srv.builtinOIDCCfg))
		r.Get("/oidc-callback", auth.BuiltinOIDCCallbackHandler(srv.builtinOIDCCfg))
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

// setupTerminalRoute configures the terminal WebSocket route
func (srv *Server) setupTerminalRoute(ctx context.Context, r *chi.Mux, apiV2BasePath string) {
	termHandler := terminal.NewHandler(
		srv.authService,
		srv.auditService,
		terminal.GetDefaultShell(),
	)

	wsPath := path.Join(apiV2BasePath, "terminal/ws")
	r.Get(wsPath, termHandler.ServeHTTP)

	logger.Info(ctx, "Terminal WebSocket route configured", slog.String("path", wsPath))
}

// startServer starts the HTTP server with or without TLS
func (srv *Server) startServer(ctx context.Context) {
	var err error

	if srv.listener != nil {
		// Use pre-bound listener (for tests or external listener management)
		if srv.config.Server.TLS != nil {
			logger.Info(ctx, "Starting TLS server on pre-bound listener",
				tag.Cert(srv.config.Server.TLS.CertFile),
				slog.String("key", srv.config.Server.TLS.KeyFile))
			err = srv.httpServer.ServeTLS(srv.listener, srv.config.Server.TLS.CertFile, srv.config.Server.TLS.KeyFile)
		} else {
			logger.Info(ctx, "Starting server on pre-bound listener")
			err = srv.httpServer.Serve(srv.listener)
		}
	} else {
		// Original behavior: create listener internally
		if srv.config.Server.TLS != nil {
			logger.Info(ctx, "Starting TLS server",
				tag.Cert(srv.config.Server.TLS.CertFile),
				slog.String("key", srv.config.Server.TLS.KeyFile))
			err = srv.httpServer.ListenAndServeTLS(srv.config.Server.TLS.CertFile, srv.config.Server.TLS.KeyFile)
		} else {
			err = srv.httpServer.ListenAndServe()
		}
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
