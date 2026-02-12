package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/eval"
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
	"github.com/dagu-org/dagu/internal/tunnel"
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
	tunnelService      *tunnel.Service
	defaultExecMode    config.ExecutionMode
	dagWritesDisabled  bool // True when git sync read-only mode is active
	agentConfigStore   agent.ConfigStore
	agentModelStore    agent.ModelStore
	agentMemoryStore   agent.MemoryStore
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

// WithTunnelService returns an APIOption that sets the API's TunnelService.
func WithTunnelService(ts *tunnel.Service) APIOption {
	return func(a *API) {
		a.tunnelService = ts
	}
}

// WithAgentConfigStore returns an APIOption that sets the API's agent config store.
func WithAgentConfigStore(store agent.ConfigStore) APIOption {
	return func(a *API) {
		a.agentConfigStore = store
	}
}

// WithAgentModelStore returns an APIOption that sets the API's agent model store.
func WithAgentModelStore(store agent.ModelStore) APIOption {
	return func(a *API) {
		a.agentModelStore = store
	}
}

// WithAgentMemoryStore returns an APIOption that sets the API's agent memory store.
func WithAgentMemoryStore(store agent.MemoryStore) APIOption {
	return func(a *API) {
		a.agentMemoryStore = store
	}
}

// New constructs an API configured with the provided stores, runtime manager,
// configuration, coordinator client, service registry, Prometheus registry,
// and resource service. It builds the remote node map and base path, then
// applies any supplied APIOption functions to customize the instance.
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
		defaultExecMode:    cfg.DefaultExecMode,
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

	baseURL = a.evaluateAndNormalizeURL(ctx, baseURL)
	swagger.Servers = append(swagger.Servers, &openapi3.Server{URL: baseURL})

	basePath := a.evaluateBasePath(ctx)
	if basePath != "" {
		swagger.Servers = append(swagger.Servers, &openapi3.Server{
			URL: path.Join(baseURL, basePath),
		})
	}

	if a.config.Server.StrictValidation {
		r.Use(a.createValidatorMiddleware(swagger))
	}

	authOptions, err := a.buildAuthOptions(ctx, basePath)
	if err != nil {
		return err
	}

	r.Group(func(r chi.Router) {
		r.Use(frontendauth.ClientIPMiddleware())
		r.Use(frontendauth.Middleware(authOptions))
		r.Use(WithRemoteNode(a.remoteNodes, a.apiBasePath))

		options := api.StrictHTTPServerOptions{
			ResponseErrorHandlerFunc: a.handleError,
		}
		handler := api.NewStrictHandlerWithOptions(a, nil, options)
		r.Mount("/", api.Handler(handler))
	})

	return nil
}

func (a *API) evaluateAndNormalizeURL(ctx context.Context, baseURL string) string {
	if evaluated, err := eval.String(ctx, baseURL, eval.WithOSExpansion()); err != nil {
		logger.Warn(ctx, "Failed to evaluate API base URL",
			tag.URL(baseURL),
			tag.Error(err))
	} else {
		baseURL = evaluated
	}
	return strings.TrimRight(baseURL, "/")
}

func (a *API) evaluateBasePath(ctx context.Context) string {
	basePath := a.config.Server.BasePath
	if evaluated, err := eval.String(ctx, basePath, eval.WithOSExpansion()); err != nil {
		logger.Warn(ctx, "Failed to evaluate server base path",
			tag.Path(basePath),
			tag.Error(err))
	} else {
		basePath = evaluated
	}
	return basePath
}

func (a *API) createValidatorMiddleware(swagger *openapi3.T) func(http.Handler) http.Handler {
	return oapimiddleware.OapiRequestValidatorWithOptions(
		swagger, &oapimiddleware.Options{
			SilenceServersWarning: true,
			Options: openapi3filter.Options{
				AuthenticationFunc: func(_ context.Context, _ *openapi3filter.AuthenticationInput) error {
					return nil
				},
			},
		},
	)
}

func (a *API) buildAuthOptions(ctx context.Context, basePath string) (frontendauth.Options, error) {
	authConfig := a.config.Server.Auth
	basicAuthEnabled := authConfig.Basic.Username != "" && authConfig.Basic.Password != ""
	authRequired := authConfig.Mode != config.AuthModeNone || basicAuthEnabled || authConfig.Token.Value != ""

	publicPaths := []string{
		pathutil.BuildPublicEndpointPath(basePath, "api/v1/health"),
		pathutil.BuildPublicEndpointPath(basePath, "api/v1/auth/login"),
	}
	if a.config.Server.Metrics == config.MetricsAccessPublic {
		publicPaths = append(publicPaths, pathutil.BuildPublicEndpointPath(basePath, "api/v1/metrics"))
	}

	authOptions := frontendauth.Options{
		Realm:            "restricted",
		APITokenEnabled:  authConfig.Token.Value != "",
		APIToken:         authConfig.Token.Value,
		BasicAuthEnabled: basicAuthEnabled,
		AuthRequired:     authRequired,
		Creds:            map[string]string{authConfig.Basic.Username: authConfig.Basic.Password},
		PublicPaths:      publicPaths,
		PublicPathPrefixes: []string{
			pathutil.BuildPublicEndpointPath(basePath, "api/v1/webhooks") + "/",
		},
	}

	if err := a.configureOIDC(ctx, authConfig.OIDC, &authOptions); err != nil {
		return frontendauth.Options{}, err
	}

	if err := a.configureBuiltinAuth(authConfig, &authOptions); err != nil {
		return frontendauth.Options{}, err
	}

	return authOptions, nil
}

func (a *API) configureOIDC(ctx context.Context, oidcConfig config.AuthOIDC, opts *frontendauth.Options) error {
	if oidcConfig.ClientID == "" || oidcConfig.ClientSecret == "" || oidcConfig.Issuer == "" {
		return nil
	}

	oidcCfg, err := frontendauth.InitVerifierAndConfig(ctx, oidcConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC: %w", err)
	}

	opts.OIDCAuthEnabled = true
	opts.OIDCWhitelist = oidcConfig.Whitelist
	opts.OIDCProvider = oidcCfg.Provider
	opts.OIDCVerify = oidcCfg.Verifier
	opts.OIDCConfig = oidcCfg.Config
	return nil
}

func (a *API) configureBuiltinAuth(authConfig config.Auth, opts *frontendauth.Options) error {
	if authConfig.Mode != config.AuthModeBuiltin {
		return nil
	}

	if a.authService == nil {
		return fmt.Errorf("builtin auth mode configured but auth service not initialized")
	}

	opts.JWTValidator = a.authService
	if a.authService.HasAPIKeyStore() {
		opts.APIKeyValidator = a.authService
	}
	return nil
}

func (a *API) handleError(w http.ResponseWriter, r *http.Request, err error) {
	code, message, httpStatusCode := a.resolveError(err)

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

func (a *API) resolveError(err error) (api.ErrorCode, string, int) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Code, apiErr.Message, apiErr.HTTPStatus
	}

	if errors.Is(err, exec.ErrDAGNotFound) {
		return api.ErrorCodeNotFound, "DAG not found", http.StatusNotFound
	}
	if errors.Is(err, exec.ErrDAGRunIDNotFound) {
		return api.ErrorCodeNotFound, "dag-run ID not found", http.StatusNotFound
	}
	if errors.Is(err, exec.ErrDAGAlreadyExists) {
		return api.ErrorCodeAlreadyExists, "DAG already exists", http.StatusConflict
	}

	return api.ErrorCodeInternalError, "An unexpected error occurred", http.StatusInternalServerError
}

func (a *API) isAllowed(perm config.Permission) error {
	if !a.config.Server.Permissions[perm] {
		return errPermissionDenied
	}
	return nil
}

// requireAdmin checks if the current user has admin role.
// Returns nil if auth is not enabled (authService is nil).
func (a *API) requireAdmin(ctx context.Context) error {
	if a.authService == nil {
		return nil
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return errAuthRequired
	}
	if !user.Role.IsAdmin() {
		return errInsufficientPermissions
	}
	return nil
}

// Predefined errors for common authorization failures.
var (
	errDAGWritesDisabled = &Error{
		HTTPStatus: http.StatusForbidden,
		Code:       api.ErrorCodeForbidden,
		Message:    "DAG modifications disabled: Git sync is in read-only mode (pushEnabled: false)",
	}
	errAuthRequired = &Error{
		HTTPStatus: http.StatusUnauthorized,
		Code:       api.ErrorCodeUnauthorized,
		Message:    "Authentication required",
	}
	errPermissionDenied = &Error{
		HTTPStatus: http.StatusForbidden,
		Code:       api.ErrorCodeForbidden,
		Message:    "Permission denied",
	}
	errInsufficientPermissions = &Error{
		HTTPStatus: http.StatusForbidden,
		Code:       api.ErrorCodeForbidden,
		Message:    "Insufficient permissions",
	}
	errUserManagementDisabled = &Error{
		HTTPStatus: http.StatusUnauthorized,
		Code:       api.ErrorCodeUnauthorized,
		Message:    "User management is not enabled",
	}
)

// requireDAGWrite checks all permissions for DAG write operations:
// 1. Server-level permission (PermissionWriteDAGs)
// 2. User role permission (CanWrite)
// 3. Git sync read-only mode (dagWritesDisabled)
func (a *API) requireDAGWrite(ctx context.Context) error {
	if !a.config.Server.Permissions[config.PermissionWriteDAGs] {
		return errPermissionDenied
	}
	if a.authService != nil {
		user, ok := auth.UserFromContext(ctx)
		if !ok {
			return errAuthRequired
		}
		if !user.Role.CanWrite() {
			return errInsufficientPermissions
		}
	}
	if a.dagWritesDisabled {
		return errDAGWritesDisabled
	}
	return nil
}

// requireExecute checks if the current user can execute (run/stop) DAGs.
// Returns nil if auth is not enabled (authService is nil).
func (a *API) requireExecute(ctx context.Context) error {
	if a.authService == nil {
		return nil
	}
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return errAuthRequired
	}
	if !user.Role.CanExecute() {
		return errInsufficientPermissions
	}
	return nil
}

// requireUserManagement checks if user management is enabled.
func (a *API) requireUserManagement() error {
	if a.authService == nil {
		return errUserManagementDisabled
	}
	return nil
}

// logAudit logs an audit entry with the specified category, action, and details.
// It silently returns if the audit service is not configured.
// User and IP are extracted from context; missing user is allowed (recorded as empty).
func (a *API) logAudit(ctx context.Context, category audit.Category, action string, details any) {
	if a.auditService == nil {
		return
	}

	var userID, username string
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		userID = user.ID
		username = user.Username
	}

	clientIP, _ := auth.ClientIPFromContext(ctx)

	var detailsStr string
	if details != nil {
		detailsJSON, err := json.Marshal(details)
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			detailsStr = "{}"
		} else {
			detailsStr = string(detailsJSON)
		}
	}

	entry := audit.NewEntry(category, action, userID, username).
		WithDetails(detailsStr).
		WithIPAddress(clientIP)

	if err := a.auditService.Log(ctx, entry); err != nil {
		logger.Warn(ctx, "Failed to write audit log",
			tag.Error(err),
			slog.String("action", action),
			slog.String("category", string(category)),
		)
	}
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
