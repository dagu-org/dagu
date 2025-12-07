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
	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/frontend/api/pathutil"
	"github.com/dagu-org/dagu/internal/service/frontend/auth"
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
}

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
) *API {
	remoteNodes := make(map[string]config.RemoteNode)
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes[n.Name] = n
	}

	return &API{
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
	if evaluatedAuth, err := cmdutil.EvalObject(ctx, authConfig, nil); err != nil {
		logger.Warn(ctx, "Failed to evaluate auth configuration", tag.Error(err))
	} else {
		authConfig = evaluatedAuth
	}
	authOptions := auth.Options{
		Realm:            "restricted",
		APITokenEnabled:  authConfig.Token.Value != "",
		APIToken:         authConfig.Token.Value,
		BasicAuthEnabled: authConfig.Basic.Username != "" && authConfig.Basic.Password != "",
		Creds:            map[string]string{authConfig.Basic.Username: authConfig.Basic.Password},
		PublicPaths: []string{
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/health"),
			pathutil.BuildPublicEndpointPath(basePath, "api/v2/metrics"),
		},
	}

	// Initialize OIDC if enabled
	authOIDC := authConfig.OIDC
	if authOIDC.ClientId != "" && authOIDC.ClientSecret != "" && authOIDC.Issuer != "" {
		oidcCfg, err := auth.InitVerifierAndConfig(authOIDC)
		if err != nil {
			return fmt.Errorf("failed to initialize OIDC: %w", err)
		}
		authOptions.OIDCAuthEnabled = true
		authOptions.OIDCWhitelist = authOIDC.Whitelist
		authOptions.OIDCProvider = oidcCfg.Provider
		authOptions.OIDCVerify = oidcCfg.Verifier
		authOptions.OIDCConfig = oidcCfg.Config
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authOptions))
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
