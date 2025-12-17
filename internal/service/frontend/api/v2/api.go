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
	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
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
	dagStore           execution.DAGStore
	dagRunStore        execution.DAGRunStore
	dagRunMgr          runtime.Manager
	queueStore         execution.QueueStore
	procStore          execution.ProcStore
	remoteNodes        map[string]config.RemoteNode
	apiBasePath        string
	logEncodingCharset string
	config             *config.Config
	metricsRegistry    *prometheus.Registry
	coordinatorCli     coordinator.Client
	serviceRegistry    execution.ServiceRegistry
	subCmdBuilder      *runtime.SubCmdBuilder
	resourceService    *resource.Service
	authService        AuthService
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
	dr execution.DAGStore,
	drs execution.DAGRunStore,
	qs execution.QueueStore,
	ps execution.ProcStore,
	drm runtime.Manager,
	cfg *config.Config,
	cc coordinator.Client,
	sr execution.ServiceRegistry,
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
	// Basic auth is disabled when builtin mode is enabled (users should use builtin auth instead)
	basicAuthEnabled := authConfig.Basic.Username != "" && authConfig.Basic.Password != "" && authConfig.Mode != config.AuthModeBuiltin
	authOptions := frontendauth.Options{
		Realm:            "restricted",
		APITokenEnabled:  authConfig.Token.Value != "",
		APIToken:         authConfig.Token.Value,
		BasicAuthEnabled: basicAuthEnabled,
		Creds:            map[string]string{authConfig.Basic.Username: authConfig.Basic.Password},
		PublicPaths: []string{
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/health"),
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/metrics"),
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/auth/login"),
		},
	}

	// Initialize OIDC if enabled
	authOIDC := authConfig.OIDC
	if authOIDC.ClientId != "" && authOIDC.ClientSecret != "" && authOIDC.Issuer != "" {
		oidcCfg, err := frontendauth.InitVerifierAndConfig(authOIDC)
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
	}

	r.Group(func(r chi.Router) {
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
	switch err := err.(type) {
	case *Error:
		apiErr = err
	case Error:
		apiErr = &err
	}

	if apiErr != nil {
		code = apiErr.Code
		message = apiErr.Message
		httpStatusCode = apiErr.HTTPStatus
	}

	switch {
	case errors.Is(err, execution.ErrDAGNotFound):
		code = api.ErrorCodeNotFound
		message = "DAG not found"

	case errors.Is(err, execution.ErrDAGRunIDNotFound):
		code = api.ErrorCodeNotFound
		message = "dag-run ID not found"

	case errors.Is(err, execution.ErrDAGAlreadyExists):
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

// requireWrite checks if the current user can write (create/edit/delete) DAGs.
// Returns nil if auth is not enabled (authService is nil).
func (a *API) requireWrite(ctx context.Context) error {
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
	if !user.Role.CanWrite() {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "Insufficient permissions",
			HTTPStatus: http.StatusForbidden,
		}
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
func toPagination[T any](paginatedResult execution.PaginatedResult[T]) api.Pagination {
	return api.Pagination{
		CurrentPage:  paginatedResult.CurrentPage,
		NextPage:     paginatedResult.NextPage,
		PrevPage:     paginatedResult.PrevPage,
		TotalPages:   paginatedResult.TotalPages,
		TotalRecords: paginatedResult.TotalCount,
	}
}
