package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/frontend/api/pathutil"
	frontendauth "github.com/dagu-org/dagu/internal/service/frontend/auth"
	"github.com/dagu-org/dagu/internal/service/resource"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var _ api.StrictServerInterface = (*API)(nil)

type API struct {
	dagStore           exec.DAGStore
	dagRunStore        exec.DAGRunStore
	dagRunMgr          runtime.Manager
	queueStore         exec.QueueStore
	procStore          exec.ProcStore
	remoteNodes        map[string]config.RemoteNode
	apiBasePath        string
	logEncodingCharset string
	config             *config.Config
	metricsRegistry    *prometheus.Registry
	coordinatorCli     coordinator.Client
	serviceRegistry    exec.ServiceRegistry
	subCmdBuilder      *runtime.SubCmdBuilder
	resourceService    *resource.Service
	authService        AuthService
	auditService       *audit.Service
	syncService        SyncService
	dagWritesDisabled  bool // True when git sync read-only mode is active
}

// AuthService defines the interface for authentication operations.
// This allows the API to work with or without auth service being configured.
type AuthService interface {
	Authenticate(ctx context.Context, username, password string) (*auth.User, error)
	GenerateToken(user *auth.User) (*authservice.TokenResult, error)
	GetUserFromToken(ctx context.Context, token string) (*auth.User, error)
	CreateUser(ctx context.Context, input authservice.CreateUserInput) (*auth.User, error)
	GetUser(ctx context.Context, id string) (*auth.User, error)
	ListUsers(ctx context.Context) ([]*auth.User, error)
	UpdateUser(ctx context.Context, id string, input authservice.UpdateUserInput) (*auth.User, error)
	DeleteUser(ctx context.Context, id string, currentUserID string) error
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	ResetPassword(ctx context.Context, userID, newPassword string) error
	// API Key management
	CreateAPIKey(ctx context.Context, input authservice.CreateAPIKeyInput, creatorID string) (*authservice.CreateAPIKeyResult, error)
	GetAPIKey(ctx context.Context, id string) (*auth.APIKey, error)
	ListAPIKeys(ctx context.Context) ([]*auth.APIKey, error)
	UpdateAPIKey(ctx context.Context, id string, input authservice.UpdateAPIKeyInput) (*auth.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	ValidateAPIKey(ctx context.Context, keySecret string) (*auth.APIKey, error)
	HasAPIKeyStore() bool
	// Webhook management
	CreateWebhook(ctx context.Context, dagName, creatorID string) (*authservice.CreateWebhookResult, error)
	GetWebhookByDAGName(ctx context.Context, dagName string) (*auth.Webhook, error)
	ListWebhooks(ctx context.Context) ([]*auth.Webhook, error)
	DeleteWebhook(ctx context.Context, dagName string) error
	RegenerateWebhookToken(ctx context.Context, dagName string) (*authservice.CreateWebhookResult, error)
	ToggleWebhook(ctx context.Context, dagName string, enabled bool) (*auth.Webhook, error)
	ValidateWebhookToken(ctx context.Context, dagName, token string) (*auth.Webhook, error)
	HasWebhookStore() bool
}

// APIOption is a functional option for configuring the API.
type APIOption func(*API)

// WithAuthService returns an APIOption that sets the API's AuthService.
// When applied, the provided AuthService will be used by API methods and middleware; passing nil disables authentication.
func WithAuthService(as AuthService) APIOption {
	return func(a *API) {
		a.authService = as
	}
}

// WithAuditService returns an APIOption that sets the API's AuditService.
func WithAuditService(as *audit.Service) APIOption {
	return func(a *API) {
		a.auditService = as
	}
}

// WithSyncService returns an APIOption that sets the API's SyncService.
func WithSyncService(ss SyncService) APIOption {
	return func(a *API) {
		a.syncService = ss
	}
}

// New constructs an API instance wired with the provided DAG, DAG-run, queue and proc stores,
// runtime manager, configuration, coordinator client, service registry, metrics registry, and resource service.
// It also builds the internal remote node map from cfg.Server.RemoteNodes and initializes the sub-command
// New constructs an *API configured with the provided stores, runtime manager,
// configuration, coordinator client, service registry, Prometheus registry,
// and resource service.
//
// It builds the API instance (including the remote node map and base path) and
// applies any supplied APIOption functions to customize the instance before
// returning it.
func New(
	dr exec.DAGStore,
	drs exec.DAGRunStore,
	qs exec.QueueStore,
	ps exec.ProcStore,
	drm runtime.Manager,
	cfg *config.Config,
	cc coordinator.Client,
	sr exec.ServiceRegistry,
	mr *prometheus.Registry,
	rs *resource.Service,
	opts ...APIOption,
) *API {
	remoteNodes := make(map[string]config.RemoteNode)
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes[n.Name] = n
	}

	a := &API{
		dagStore:           dr,
		dagRunStore:        drs,
		queueStore:         qs,
		procStore:          ps,
		dagRunMgr:          drm,
		logEncodingCharset: cfg.UI.LogEncodingCharset,
		remoteNodes:        remoteNodes,
		subCmdBuilder:      runtime.NewSubCmdBuilder(cfg),
		apiBasePath:        cfg.Server.APIBasePath,
		config:             cfg,
		coordinatorCli:     cc,
		serviceRegistry:    sr,
		metricsRegistry:    mr,
		resourceService:    rs,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Set read-only mode flag based on git sync config
	// When enabled with push disabled, DAG write operations are blocked
	a.dagWritesDisabled = cfg.GitSync.Enabled && !cfg.GitSync.PushEnabled

	return a
}

func (a *API) ConfigureRoutes(ctx context.Context, r chi.Router, baseURL string) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to get swagger: %w", err)
	}

	// Create a list of server URLs
	if evaluatedBaseURL, err := cmdutil.EvalString(ctx, baseURL); err != nil {
		logger.Warn(ctx, "Failed to evaluate API base URL",
			tag.URL(baseURL),
			tag.Error(err))
	} else {
		baseURL = evaluatedBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	serverURLs := []string{baseURL}

	// Set the server URLs in the swagger spec
	for _, url := range serverURLs {
		swagger.Servers = append(swagger.Servers, &openapi3.Server{URL: url})
	}

	// Setup the API base path
	basePath := a.config.Server.BasePath
	if evaluatedBasePath, err := cmdutil.EvalString(ctx, basePath); err != nil {
		logger.Warn(ctx, "Failed to evaluate server base path",
			tag.Path(basePath),
			tag.Error(err))
	} else {
		basePath = evaluatedBasePath
	}
	if basePath != "" {
		swagger.Servers = append(swagger.Servers, &openapi3.Server{
			URL: path.Join(baseURL, basePath),
		})
	}

	// Create the oapi-codegen validator middleware
	if a.config.Server.StrictValidation {
		// It is problematic to use the validator behind a reverse proxy
		validator := oapimiddleware.OapiRequestValidatorWithOptions(
			swagger, &oapimiddleware.Options{
				SilenceServersWarning: true,
				Options: openapi3filter.Options{
					AuthenticationFunc: func(_ context.Context, _ *openapi3filter.AuthenticationInput) error {
						return nil
					},
				},
			},
		)
		r.Use(validator)
	}

	options := api.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: a.handleError,
	}

	// Initialize auth configuration
	authConfig := a.config.Server.Auth
	// Basic auth works independently if credentials are configured
	basicAuthEnabled := authConfig.Basic.Username != "" && authConfig.Basic.Password != ""
	// Auth is required unless mode is explicitly set to "none" and no credentials are configured
	authRequired := authConfig.Mode != config.AuthModeNone || basicAuthEnabled || authConfig.Token.Value != ""
	// Build public paths list - metrics is only public if explicitly configured
	publicPaths := []string{
		pathutil.BuildPublicEndpointPath(basePath, "api/v2/health"),
		pathutil.BuildPublicEndpointPath(basePath, "api/v2/auth/login"),
	}
	if a.config.Server.Metrics == config.MetricsAccessPublic {
		publicPaths = append(publicPaths, pathutil.BuildPublicEndpointPath(basePath, "api/v2/metrics"))
	}

	authOptions := frontendauth.Options{
		Realm:            "restricted",
		APITokenEnabled:  authConfig.Token.Value != "",
		APIToken:         authConfig.Token.Value,
		BasicAuthEnabled: basicAuthEnabled,
		AuthRequired:     authRequired,
		Creds:            map[string]string{authConfig.Basic.Username: authConfig.Basic.Password},
		PublicPaths:      publicPaths,
		// Webhook trigger endpoints use their own authentication (DAG-specific token)
		// Note: We must append "/" to ensure only /api/v2/webhooks/{fileName} is public,
		// not /api/v2/webhooks itself (the list endpoint which requires admin auth)
		PublicPathPrefixes: []string{
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/webhooks") + "/",
		},
	}

	// Initialize OIDC if enabled
	authOIDC := authConfig.OIDC
	if authOIDC.ClientId != "" && authOIDC.ClientSecret != "" && authOIDC.Issuer != "" {
		oidcCfg, err := frontendauth.InitVerifierAndConfig(ctx, authOIDC)
		if err != nil {
			return fmt.Errorf("failed to initialize OIDC: %w", err)
		}
		authOptions.OIDCAuthEnabled = true
		authOptions.OIDCWhitelist = authOIDC.Whitelist
		authOptions.OIDCProvider = oidcCfg.Provider
		authOptions.OIDCVerify = oidcCfg.Verifier
		authOptions.OIDCConfig = oidcCfg.Config
	}

	// Apply authentication middleware
	// For builtin mode, we need to add JWT validation in addition to other auth methods
	if authConfig.Mode == config.AuthModeBuiltin {
		if a.authService == nil {
			return fmt.Errorf("builtin auth mode configured but auth service not initialized")
		}
		// Add JWT as an additional auth method for builtin mode
		authOptions.JWTValidator = a.authService
		// Add API key validation for builtin mode (if API key store is configured)
		if a.authService.HasAPIKeyStore() {
			authOptions.APIKeyValidator = a.authService
		}
	}

	r.Group(func(r chi.Router) {
		// Add client IP to context for audit logging (must be before auth middleware)
		r.Use(frontendauth.ClientIPMiddleware())
		// Use the unified middleware that handles all auth methods:
		// - Basic auth (if configured)
		// - API token (if configured)
		// - OIDC (if configured)
		// - JWT tokens (if builtin mode with JWTValidator)
		r.Use(frontendauth.Middleware(authOptions))
		r.Use(WithRemoteNode(a.remoteNodes, a.apiBasePath))

		handler := api.NewStrictHandlerWithOptions(a, nil, options)
		r.Mount("/", api.Handler(handler))
	})

	return nil
}

func (a *API) handleError(w http.ResponseWriter, r *http.Request, err error) {
	code := api.ErrorCodeInternalError
	message := "An unexpected error occurred"
	httpStatusCode := http.StatusInternalServerError

	var apiErr *Error
	if errors.As(err, &apiErr) {
		code = apiErr.Code
		message = apiErr.Message
		httpStatusCode = apiErr.HTTPStatus
	}

	switch {
	case errors.Is(err, exec.ErrDAGNotFound):
		code = api.ErrorCodeNotFound
		message = "DAG not found"
	case errors.Is(err, exec.ErrDAGRunIDNotFound):
		code = api.ErrorCodeNotFound
		message = "dag-run ID not found"
	case errors.Is(err, exec.ErrDAGAlreadyExists):
		code = api.ErrorCodeAlreadyExists
		message = "DAG already exists"
	}

	if httpStatusCode == http.StatusInternalServerError {
		logger.Errorf(r.Context(), "Internal server error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(api.Error{
		Code:    code,
		Message: message,
	})
}

func (a *API) isAllowed(perm config.Permission) error {
	if !a.config.Server.Permissions[perm] {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Permission denied",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

// requireAdmin checks if the current user has admin role.
// Returns nil if auth is not enabled (authService is nil).
func (a *API) requireAdmin(ctx context.Context) error {
	if a.authService == nil {
		return nil // Auth not enabled, allow access
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "Authentication required",
			HTTPStatus: http.StatusUnauthorized,
		}
	}
	if !user.Role.IsAdmin() {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Insufficient permissions",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

// errDAGWritesDisabled is returned when DAG modifications are blocked in read-only mode.
var errDAGWritesDisabled = &Error{
	HTTPStatus: http.StatusForbidden,
	Code:       api.ErrorCodeForbidden,
	Message:    "DAG modifications disabled: Git sync is in read-only mode (pushEnabled: false)",
}

// requireDAGWrite checks all permissions for DAG write operations:
// 1. Server-level permission (PermissionWriteDAGs)
// 2. User role permission (CanWrite)
// 3. Git sync read-only mode (dagWritesDisabled)
func (a *API) requireDAGWrite(ctx context.Context) error {
	// Check server-level permission
	if !a.config.Server.Permissions[config.PermissionWriteDAGs] {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Permission denied",
			HTTPStatus: http.StatusForbidden,
		}
	}

	// Check user role permission (if auth enabled)
	if a.authService != nil {
		user, ok := auth.UserFromContext(ctx)
		if !ok {
			return &Error{
				Code:       api.ErrorCodeUnauthorized,
				Message:    "Authentication required",
				HTTPStatus: http.StatusUnauthorized,
			}
		}
		if !user.Role.CanWrite() {
			return &Error{
				Code:       api.ErrorCodeForbidden,
				Message:    "Insufficient permissions",
				HTTPStatus: http.StatusForbidden,
			}
		}
	}

	// Check git sync read-only mode
	if a.dagWritesDisabled {
		return errDAGWritesDisabled
	}

	return nil
}

// requireExecute checks if the current user can execute (run/stop) DAGs.
// Returns nil if auth is not enabled (authService is nil).
func (a *API) requireExecute(ctx context.Context) error {
	if a.authService == nil {
		return nil // Auth not enabled, allow access
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "Authentication required",
			HTTPStatus: http.StatusUnauthorized,
		}
	}
	if !user.Role.CanExecute() {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Insufficient permissions",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

// requireUserManagement checks if user management is enabled.
func (a *API) requireUserManagement() error {
	if a.authService == nil {
		return &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "User management is not enabled",
			HTTPStatus: http.StatusUnauthorized,
		}
	}
	return nil
}

// ptrOf returns a pointer to v, or nil if v is the zero value for its type.
func ptrOf[T any](v T) *T {
	if reflect.ValueOf(v).IsZero() {
		return nil
	}

	return &v
}

func valueOf[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// toPagination converts a paginated result to an API pagination object.
func toPagination[T any](paginatedResult exec.PaginatedResult[T]) api.Pagination {
	return api.Pagination{
		CurrentPage:  paginatedResult.CurrentPage,
		NextPage:     paginatedResult.NextPage,
		PrevPage:     paginatedResult.PrevPage,
		TotalPages:   paginatedResult.TotalPages,
		TotalRecords: paginatedResult.TotalCount,
	}
}
