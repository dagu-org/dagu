package frontend

import (
	"context"
	"encoding/json"
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/dagu-org/dagu/internal/agent"
	authmodel "github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	cmnschema "github.com/dagu-org/dagu/internal/cmn/schema"
	"github.com/dagu-org/dagu/internal/cmn/telemetry"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/gitsync"
	_ "github.com/dagu-org/dagu/internal/llm/allproviders" // Register LLM providers
	"github.com/dagu-org/dagu/internal/persis/fileagentconfig"
	"github.com/dagu-org/dagu/internal/persis/fileapikey"
	"github.com/dagu-org/dagu/internal/persis/fileaudit"
	"github.com/dagu-org/dagu/internal/persis/fileconversation"
	"github.com/dagu-org/dagu/internal/persis/fileuser"
	"github.com/dagu-org/dagu/internal/persis/filewebhook"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	apiv1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/dagu-org/dagu/internal/service/frontend/auth"
	"github.com/dagu-org/dagu/internal/service/frontend/metrics"
	"github.com/dagu-org/dagu/internal/service/frontend/sse"
	"github.com/dagu-org/dagu/internal/service/frontend/terminal"
	"github.com/dagu-org/dagu/internal/service/oidcprovision"
	"github.com/dagu-org/dagu/internal/service/resource"
	"github.com/dagu-org/dagu/internal/tunnel"
	"github.com/dagu-org/dagu/internal/upgrade"
)

// Server represents the HTTP server for the frontend application.
type Server struct {
	apiV1            *apiv1.API
	agentAPI         *agent.API
	agentConfigStore *fileagentconfig.Store
	config           *config.Config
	httpServer       *http.Server
	funcsConfig      funcsConfig
	builtinOIDCCfg   *auth.BuiltinOIDCConfig
	authService      *authservice.Service
	auditService     *audit.Service
	syncService      gitsync.Service
	listener         net.Listener
	sseHub           *sse.Hub
	metricsRegistry  *prometheus.Registry
	tunnelAPIOpts    []apiv1.APIOption
	dagStore         exec.DAGStore
}

// ServerOption is a functional option for configuring the Server.
type ServerOption func(*Server)

// WithListener sets a pre-bound listener for the server (useful for tests).
func WithListener(l net.Listener) ServerOption {
	return func(s *Server) {
		s.listener = l
	}
}

// WithTunnelService enables real-time tunnel status via the API.
func WithTunnelService(ts *tunnel.Service) ServerOption {
	return func(s *Server) {
		if ts != nil {
			s.tunnelAPIOpts = append(s.tunnelAPIOpts, apiv1.WithTunnelService(ts))
		}
	}
}

// NewServer constructs a Server from the provided configuration, stores, and services.
// Returns an error if initialization fails (e.g., when builtin auth fails to initialize).
func NewServer(ctx context.Context, cfg *config.Config, dr exec.DAGStore, drs exec.DAGRunStore, qs exec.QueueStore, ps exec.ProcStore, drm runtime.Manager, cc coordinator.Client, sr exec.ServiceRegistry, mr *prometheus.Registry, collector *telemetry.Collector, rs *resource.Service, opts ...ServerOption) (*Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	remoteNodes := make([]string, 0, len(cfg.Server.RemoteNodes))
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes = append(remoteNodes, n.Name)
	}

	var (
		apiOpts         []apiv1.APIOption
		builtinOIDCCfg  *auth.BuiltinOIDCConfig
		oidcEnabled     bool
		oidcButtonLabel string
	)

	auditSvc, err := initAuditService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audit service: %w", err)
	}
	if auditSvc != nil {
		apiOpts = append(apiOpts, apiv1.WithAuditService(auditSvc))
	}

	syncSvc := initSyncService(ctx, cfg)
	if syncSvc != nil {
		apiOpts = append(apiOpts, apiv1.WithSyncService(syncSvc))
	}

	agentConfigStore, err := fileagentconfig.New(cfg.Paths.DataDir)
	if err != nil {
		logger.Warn(ctx, "Failed to create agent config store", tag.Error(err))
	}

	var agentAPI *agent.API
	if agentConfigStore != nil {
		agentAPI, err = initAgentAPI(ctx, agentConfigStore, &cfg.Paths, dr, auditSvc)
		if err != nil {
			logger.Warn(ctx, "Failed to initialize agent API", tag.Error(err))
		}
	}

	var authSvc *authservice.Service
	if cfg.Server.Auth.Mode == config.AuthModeBuiltin {
		result, err := initBuiltinAuthService(cfg, collector)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize builtin auth service: %w", err)
		}
		authSvc = result.AuthService
		apiOpts = append(apiOpts, apiv1.WithAuthService(result.AuthService))

		oidcCfg := cfg.Server.Auth.OIDC
		if oidcCfg.IsConfigured() {
			oidcEnabled = true
			oidcButtonLabel = oidcCfg.ButtonLabel

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

	// Check for updates asynchronously (populates cache for next startup)
	go func() { _, _ = upgrade.CheckAndUpdateCache(config.Version) }()

	updateAvailable, latestVersion := getUpdateInfo()

	srv := &Server{
		config:           cfg,
		agentAPI:         agentAPI,
		agentConfigStore: agentConfigStore,
		builtinOIDCCfg:   builtinOIDCCfg,
		authService:      authSvc,
		auditService:     auditSvc,
		syncService:      syncSvc,
		metricsRegistry:  mr,
		dagStore:         dr,
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
			UpdateAvailable:       updateAvailable,
			LatestVersion:         latestVersion,
			AgentEnabledChecker:   agentConfigStore,
		},
	}

	for _, opt := range opts {
		opt(srv)
	}

	allAPIOptions := append(apiOpts, srv.tunnelAPIOpts...)
	if srv.agentConfigStore != nil {
		allAPIOptions = append(allAPIOptions, apiv1.WithAgentConfigStore(srv.agentConfigStore))
	}

	srv.apiV1 = apiv1.New(dr, drs, qs, ps, drm, cfg, cc, sr, mr, rs, allAPIOptions...)

	return srv, nil
}

// getUpdateInfo returns update availability and latest version from cache.
func getUpdateInfo() (updateAvailable bool, latestVersion string) {
	cache := upgrade.GetCachedUpdateInfo()
	if cache == nil {
		return false, ""
	}
	return cache.UpdateAvailable, cache.LatestVersion
}

type builtinAuthResult struct {
	AuthService *authservice.Service
	UserStore   authmodel.UserStore
}

// initBuiltinAuthService creates a file-based user store and authentication service.
func initBuiltinAuthService(cfg *config.Config, collector *telemetry.Collector) (*builtinAuthResult, error) {
	ctx := context.Background()

	if cfg.Server.Auth.Builtin.Token.Secret == "" {
		return nil, fmt.Errorf("builtin auth requires a non-empty token secret (set DAGU_AUTH_TOKEN_SECRET or server.auth.builtin.token.secret)")
	}

	userStore, err := fileuser.New(cfg.Paths.UsersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create user store: %w", err)
	}

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

	webhookCache := fileutil.NewCache[*authmodel.Webhook]("webhook", cacheLimits.Webhook.Limit, cacheLimits.Webhook.TTL)
	webhookCache.StartEviction(ctx)
	if collector != nil {
		collector.RegisterCache(webhookCache)
	}
	webhookStore, err := filewebhook.New(cfg.Paths.WebhooksDir, filewebhook.WithFileCache(webhookCache))
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook store: %w", err)
	}

	authSvc := authservice.New(userStore, authservice.Config{
		TokenSecret: cfg.Server.Auth.Builtin.Token.Secret,
		TokenTTL:    cfg.Server.Auth.Builtin.Token.TTL,
	},
		authservice.WithAPIKeyStore(apiKeyStore),
		authservice.WithWebhookStore(webhookStore),
	)

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
			// Auto-generated password: print to stdout (not structured logs)
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
func initAuditService(cfg *config.Config) (*audit.Service, error) {
	if !cfg.Server.Audit.Enabled {
		return nil, nil
	}

	store, err := fileaudit.New(filepath.Join(cfg.Paths.AdminLogsDir, "audit"))
	if err != nil {
		return nil, fmt.Errorf("failed to create audit store: %w", err)
	}

	return audit.New(store), nil
}

// initSyncService creates and returns a Git sync service if enabled.
func initSyncService(ctx context.Context, cfg *config.Config) gitsync.Service {
	if !cfg.GitSync.Enabled {
		return nil
	}

	syncCfg := gitsync.NewConfigFromGlobal(cfg.GitSync)
	svc := gitsync.NewService(syncCfg, cfg.Paths.DAGsDir, cfg.Paths.DataDir)

	if syncCfg.AutoSync.Enabled {
		if err := svc.Start(ctx); err != nil {
			logger.Error(ctx, "Failed to start git sync auto-sync", tag.Error(err))
		} else {
			logger.Info(ctx, "Git sync auto-sync started",
				slog.String("repository", syncCfg.Repository),
				slog.String("branch", syncCfg.Branch),
				slog.Int("interval", syncCfg.AutoSync.Interval))
		}
	}

	logger.Info(ctx, "Git sync service initialized",
		slog.String("repository", syncCfg.Repository),
		slog.String("branch", syncCfg.Branch))

	return svc
}

// initAgentAPI creates and returns an agent API.
// The API uses the config store to check enabled status and get provider dynamically.
func initAgentAPI(ctx context.Context, store *fileagentconfig.Store, paths *config.PathsConfig, dagStore exec.DAGStore, auditSvc *audit.Service) (*agent.API, error) {
	convStore, err := fileconversation.New(paths.ConversationsDir)
	if err != nil {
		logger.Warn(ctx, "Failed to create conversation store, persistence disabled", tag.Error(err))
	}

	hooks := agent.NewHooks()
	if auditSvc != nil {
		hooks.OnAfterToolExec(newAgentAuditHook(auditSvc))
	}

	api := agent.NewAPI(agent.APIConfig{
		ConfigStore:       store,
		WorkingDir:        paths.DAGsDir,
		Logger:            slog.Default(),
		ConversationStore: convStore,
		DAGStore:          dagStore,
		Hooks:             hooks,
		Environment: agent.EnvironmentInfo{
			DAGsDir:        paths.DAGsDir,
			LogDir:         paths.LogDir,
			DataDir:        paths.DataDir,
			ConfigFile:     paths.ConfigFileUsed,
			WorkingDir:     paths.DAGsDir,
			BaseConfigFile: paths.BaseConfig,
		},
	})

	logger.Info(ctx, "Agent API initialized")

	return api, nil
}

// newAgentAuditHook returns a hook that logs agent tool executions to the audit service.
func newAgentAuditHook(auditSvc *audit.Service) agent.AfterToolExecHookFunc {
	return func(_ context.Context, info agent.ToolExecInfo, result agent.ToolOut) {
		if info.Audit == nil {
			return // tool opted out of audit
		}

		details := make(map[string]any)
		if info.Audit.DetailExtractor != nil {
			details = info.Audit.DetailExtractor(info.Input)
		}
		if result.IsError {
			details["failed"] = true
		}
		details["conversation_id"] = info.ConversationID

		detailsJSON, _ := json.Marshal(details)
		entry := audit.NewEntry(audit.CategoryAgent, info.Audit.Action, info.UserID, info.Username).
			WithDetails(string(detailsJSON)).
			WithIPAddress(info.IPAddress)
		_ = auditSvc.Log(context.Background(), entry)
	}
}

// sanitizedRequestLogger wraps httplog's RequestLogger with URL sanitization
// to redact tokens in query strings.
func sanitizedRequestLogger(httpLogger *httplog.Logger) func(next http.Handler) http.Handler {
	loggerMiddleware := httplog.RequestLogger(httpLogger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logReq := redactTokenFromRequest(r)

			// Pass original request to next handler, but redacted request to logger
			passthrough := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				next.ServeHTTP(w, r)
			})

			loggerMiddleware(passthrough).ServeHTTP(w, logReq)
		})
	}
}

// redactTokenFromRequest returns a request with the token query parameter redacted.
// If no token is present, the original request is returned unchanged.
func redactTokenFromRequest(r *http.Request) *http.Request {
	if r.URL.RawQuery == "" || !strings.Contains(r.URL.RawQuery, "token=") {
		return r
	}

	q := r.URL.Query()
	if !q.Has("token") {
		return r
	}

	redacted := r.Clone(r.Context())
	q.Set("token", "[REDACTED]")
	redacted.URL.RawQuery = q.Encode()
	redacted.RequestURI = redacted.URL.RequestURI()

	return redacted
}

// Serve starts the HTTP server and configures routes.
func (srv *Server) Serve(ctx context.Context) error {
	logLevel := slog.LevelInfo
	if srv.config.Core.Debug {
		logLevel = slog.LevelDebug
	}

	requestLogger := httplog.NewLogger("http", httplog.Options{
		LogLevel:         logLevel,
		JSON:             srv.config.Core.LogFormat == "json",
		Concise:          true,
		RequestHeaders:   srv.config.Core.Debug,
		MessageFieldName: "msg",
		ResponseHeaders:  false,
		QuietDownRoutes:  []string{"/api/v1/events"},
		QuietDownPeriod:  10 * time.Second,
	})

	r := chi.NewMux()
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))
	r.Use(sanitizedRequestLogger(requestLogger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Content-Encoding", "Accept"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RedirectSlashes)

	apiV1BasePath := srv.configureAPIPath()
	scheme := srv.getScheme()

	if err := srv.setupRoutes(ctx, r); err != nil {
		return err
	}

	if err := srv.setupAPIRoutes(ctx, r, apiV1BasePath, scheme); err != nil {
		return err
	}

	if srv.config.Server.Terminal.Enabled && srv.authService != nil {
		srv.setupTerminalRoute(ctx, r, apiV1BasePath)
	}

	if srv.agentAPI != nil && srv.agentConfigStore != nil {
		srv.setupAgentRoutes(ctx, r)
	}

	srv.setupSSERoute(ctx, r, apiV1BasePath)

	addr := net.JoinHostPort(srv.config.Server.Host, strconv.Itoa(srv.config.Server.Port))
	srv.httpServer = &http.Server{
		Handler:           r,
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	metrics.StartUptime(ctx)
	logger.Info(ctx, "Server is starting", tag.Addr(addr))

	go srv.startServer(ctx)
	srv.setupGracefulShutdown(ctx)

	return nil
}

func (srv *Server) configureAPIPath() string {
	apiV1BasePath := path.Join(srv.config.Server.BasePath, "api/v1")
	return ensureLeadingSlash(apiV1BasePath)
}

func (srv *Server) getScheme() string {
	if srv.config.Server.TLS == nil {
		return "http"
	}
	return "https"
}

// ensureLeadingSlash ensures the path starts with a forward slash.
func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func (srv *Server) setupRoutes(ctx context.Context, r *chi.Mux) error {
	if srv.config.Server.Headless {
		logger.Info(ctx, "Headless mode enabled: UI is disabled, but API remains active")
		return nil
	}

	basePath := srv.evaluateBasePath(ctx)
	srv.setupAssetRoutes(r, basePath)
	srv.setupOIDCRoutes(r)

	oidcAuthOptions, err := srv.buildOIDCAuthOptions(ctx)
	if err != nil {
		return err
	}

	indexHandler := srv.useTemplate(ctx, "index.gohtml", "index")
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

func (srv *Server) evaluateBasePath(ctx context.Context) string {
	basePath := srv.config.Server.BasePath
	evaluated, err := eval.String(ctx, basePath, eval.WithOSExpansion())
	if err != nil {
		logger.Warn(ctx, "Failed to evaluate server base path", tag.Path(basePath), tag.Error(err))
		return basePath
	}
	return evaluated
}

func (srv *Server) setupAssetRoutes(r *chi.Mux, basePath string) {
	assetsPath := ensureLeadingSlash(path.Join(strings.TrimRight(basePath, "/"), "assets/*"))

	fileServer := http.FileServer(http.FS(assetsFS))
	if basePath != "" && basePath != "/" {
		fileServer = http.StripPrefix(strings.TrimRight(basePath, "/"), fileServer)
	}

	r.Get(assetsPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")

		// Serve schema from shared package instead of embedded assets
		if strings.HasSuffix(r.URL.Path, "dag.schema.json") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(cmnschema.DAGSchemaJSON)
			return
		}

		if ctype := mime.TypeByExtension(path.Ext(r.URL.Path)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (srv *Server) setupOIDCRoutes(r *chi.Mux) {
	if srv.builtinOIDCCfg == nil {
		return
	}
	r.Get("/oidc-login", auth.BuiltinOIDCLoginHandler(srv.builtinOIDCCfg))
	r.Get("/oidc-callback", auth.BuiltinOIDCCallbackHandler(srv.builtinOIDCCfg))
}

func (srv *Server) buildOIDCAuthOptions(ctx context.Context) (*auth.Options, error) {
	authCfg := srv.config.Server.Auth
	oidcCfg := authCfg.OIDC

	if authCfg.Mode != config.AuthModeOIDC {
		return nil, nil
	}
	if oidcCfg.ClientID == "" || oidcCfg.ClientSecret == "" || oidcCfg.Issuer == "" {
		return nil, nil
	}

	verifierCfg, err := auth.InitVerifierAndConfig(ctx, oidcCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC: %w", err)
	}

	return &auth.Options{
		OIDCAuthEnabled: true,
		OIDCWhitelist:   oidcCfg.Whitelist,
		OIDCProvider:    verifierCfg.Provider,
		OIDCVerify:      verifierCfg.Verifier,
		OIDCConfig:      verifierCfg.Config,
	}, nil
}

func (srv *Server) setupAPIRoutes(ctx context.Context, r *chi.Mux, apiV1BasePath, scheme string) error {
	var setupErr error
	r.Route(apiV1BasePath, func(r chi.Router) {
		url := fmt.Sprintf("%s://%s:%d%s", scheme, srv.config.Server.Host, srv.config.Server.Port, apiV1BasePath)
		if err := srv.apiV1.ConfigureRoutes(ctx, r, url); err != nil {
			logger.Error(ctx, "Failed to configure API routes", tag.Error(err))
			setupErr = err
		}
	})
	return setupErr
}

func (srv *Server) setupTerminalRoute(ctx context.Context, r *chi.Mux, apiV1BasePath string) {
	termHandler := terminal.NewHandler(srv.authService, srv.auditService, terminal.GetDefaultShell())
	wsPath := path.Join(apiV1BasePath, "terminal/ws")
	r.Get(wsPath, termHandler.ServeHTTP)
	logger.Info(ctx, "Terminal WebSocket route configured", slog.String("path", wsPath))
}

func (srv *Server) setupSSERoute(ctx context.Context, r *chi.Mux, apiV1BasePath string) {
	var sseMetrics *sse.Metrics
	if srv.metricsRegistry != nil {
		sseMetrics = sse.NewMetrics(srv.metricsRegistry)
	}

	srv.sseHub = sse.NewHub(sse.WithMetrics(sseMetrics))
	srv.sseHub.Start()
	srv.registerSSEFetchers()

	remoteNodes := make(map[string]config.RemoteNode)
	for _, n := range srv.config.Server.RemoteNodes {
		remoteNodes[n.Name] = n
	}

	handler := sse.NewHandler(srv.sseHub, remoteNodes, srv.authService)

	r.Get(path.Join(apiV1BasePath, "events/dags"), handler.HandleDAGsListEvents)
	r.Get(path.Join(apiV1BasePath, "events/dags/{fileName}"), handler.HandleDAGEvents)
	r.Get(path.Join(apiV1BasePath, "events/dags/{fileName}/dag-runs"), handler.HandleDAGHistoryEvents)
	r.Get(path.Join(apiV1BasePath, "events/dag-runs"), handler.HandleDAGRunsListEvents)
	r.Get(path.Join(apiV1BasePath, "events/dag-runs/{name}/{dagRunId}"), handler.HandleDAGRunEvents)
	r.Get(path.Join(apiV1BasePath, "events/dag-runs/{name}/{dagRunId}/logs"), handler.HandleDAGRunLogsEvents)
	r.Get(path.Join(apiV1BasePath, "events/dag-runs/{name}/{dagRunId}/logs/steps/{stepName}"), handler.HandleStepLogEvents)
	r.Get(path.Join(apiV1BasePath, "events/queues"), handler.HandleQueuesListEvents)
	r.Get(path.Join(apiV1BasePath, "events/queues/{name}/items"), handler.HandleQueueItemsEvents)

	logger.Info(ctx, "SSE routes configured", slog.String("basePath", apiV1BasePath))
}

func (srv *Server) registerSSEFetchers() {
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAG, srv.apiV1.GetDAGDetailsData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAGHistory, srv.apiV1.GetDAGHistoryData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAGsList, srv.apiV1.GetDAGsListData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAGRun, srv.apiV1.GetDAGRunDetailsData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAGRuns, srv.apiV1.GetDAGRunsListData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeDAGRunLogs, srv.apiV1.GetDAGRunLogsData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeStepLog, srv.apiV1.GetStepLogData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeQueues, srv.apiV1.GetQueuesListData)
	srv.sseHub.RegisterFetcher(sse.TopicTypeQueueItems, srv.apiV1.GetQueueItemsData)
}

func (srv *Server) setupAgentRoutes(ctx context.Context, r *chi.Mux) {
	authMiddleware := srv.buildAgentAuthMiddleware(ctx)
	srv.agentAPI.RegisterRoutes(r, authMiddleware)
	logger.Info(ctx, "Agent API routes configured")
}

func (srv *Server) buildAgentAuthMiddleware(_ context.Context) func(http.Handler) http.Handler {
	authOptions := srv.buildAgentAuthOptions()

	return func(next http.Handler) http.Handler {
		baseMiddleware := auth.ClientIPMiddleware()(auth.Middleware(authOptions)(next))

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token := r.URL.Query().Get("token"); token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			baseMiddleware.ServeHTTP(w, r)
		})
	}
}

func (srv *Server) buildAgentAuthOptions() auth.Options {
	authCfg := srv.config.Server.Auth

	opts := auth.Options{
		Realm:        "Dagu Agent",
		AuthRequired: authCfg.Mode != config.AuthModeNone,
	}

	if authCfg.Basic.Username != "" && authCfg.Basic.Password != "" {
		opts.BasicAuthEnabled = true
		opts.Creds = map[string]string{authCfg.Basic.Username: authCfg.Basic.Password}
	}

	if authCfg.Token.Value != "" {
		opts.APITokenEnabled = true
		opts.APIToken = authCfg.Token.Value
	}

	if authCfg.Mode == config.AuthModeBuiltin && srv.authService != nil {
		opts.JWTValidator = srv.authService
		if srv.authService.HasAPIKeyStore() {
			opts.APIKeyValidator = srv.authService
		}
	}

	return opts
}

func (srv *Server) startServer(ctx context.Context) {
	tlsCfg := srv.config.Server.TLS
	hasListener := srv.listener != nil

	if tlsCfg != nil {
		logger.Info(ctx, "Starting TLS server",
			tag.Cert(tlsCfg.CertFile), slog.String("key", tlsCfg.KeyFile),
			slog.Bool("preBoundListener", hasListener))
	} else if hasListener {
		logger.Info(ctx, "Starting server on pre-bound listener")
	}

	err := srv.serveHTTP(tlsCfg, hasListener)
	if err != nil && err != http.ErrServerClosed {
		logger.Error(ctx, "Server failed to start or unexpected shutdown", tag.Error(err))
	}
}

func (srv *Server) serveHTTP(tlsCfg *config.TLSConfig, hasListener bool) error {
	switch {
	case hasListener && tlsCfg != nil:
		return srv.httpServer.ServeTLS(srv.listener, tlsCfg.CertFile, tlsCfg.KeyFile)
	case hasListener:
		return srv.httpServer.Serve(srv.listener)
	case tlsCfg != nil:
		return srv.httpServer.ListenAndServeTLS(tlsCfg.CertFile, tlsCfg.KeyFile)
	default:
		return srv.httpServer.ListenAndServe()
	}
}

// Shutdown gracefully shuts down the server.
func (srv *Server) Shutdown(ctx context.Context) error {
	if srv.syncService != nil {
		if err := srv.syncService.Stop(); err != nil {
			logger.Warn(ctx, "Failed to stop git sync service", tag.Error(err))
		}
	}

	if srv.sseHub != nil {
		srv.sseHub.Shutdown()
		logger.Info(ctx, "SSE hub shut down")
	}

	if srv.httpServer == nil {
		return nil
	}

	logger.Info(ctx, "Server is shutting down", tag.Addr(srv.httpServer.Addr))

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	srv.httpServer.SetKeepAlivesEnabled(false)
	return srv.httpServer.Shutdown(shutdownCtx)
}

func (srv *Server) setupGracefulShutdown(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

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
