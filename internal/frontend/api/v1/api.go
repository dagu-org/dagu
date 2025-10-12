package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/frontend/auth"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
)

var _ api.StrictServerInterface = (*API)(nil)

type API struct {
	dagStore           execution.DAGStore
	dagRunStore        execution.DAGRunStore
	dagRunManager      dagrun.Manager
	remoteNodes        map[string]config.RemoteNode
	apiBasePath        string
	logEncodingCharset string
	config             *config.Config
	subCmdBuilder      *dagrun.SubCmdBuilder
}

func New(
	dr execution.DAGStore,
	drs execution.DAGRunStore,
	drm dagrun.Manager,
	cfg *config.Config,
) *API {
	remoteNodes := make(map[string]config.RemoteNode)
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes[n.Name] = n
	}

	return &API{
		dagStore:           dr,
		dagRunStore:        drs,
		dagRunManager:      drm,
		logEncodingCharset: cfg.UI.LogEncodingCharset,
		remoteNodes:        remoteNodes,
		apiBasePath:        cfg.Server.APIBasePath,
		config:             cfg,
		subCmdBuilder:      dagrun.NewSubCmdBuilder(cfg),
	}
}

func (a *API) ConfigureRoutes(r chi.Router, baseURL string) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to get swagger: %w", err)
	}

	// Create a list of server URLs
	serverURLs := []string{baseURL}

	// Set the server URLs in the swagger spec
	for _, url := range serverURLs {
		swagger.Servers = append(swagger.Servers, &openapi3.Server{URL: url})
	}

	// Create the oapi-codegen validator middleware
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

	options := api.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: a.handleError,
	}

	r.Group(func(r chi.Router) {
		authConfig := a.config.Server.Auth
		authOptions := auth.Options{
			Realm:            "restricted",
			APITokenEnabled:  authConfig.Token.Value != "",
			APIToken:         authConfig.Token.Value,
			BasicAuthEnabled: authConfig.Basic.Username != "" && authConfig.Basic.Password != "",
			Creds: map[string]string{
				authConfig.Basic.Username: authConfig.Basic.Password,
			},
		}
		r.Use(auth.Middleware(authOptions))
		r.Use(WithRemoteNode(a.remoteNodes, a.apiBasePath))

		handler := api.NewStrictHandlerWithOptions(a, nil, options)
		r.Mount("/", api.Handler(handler))
	})

	return nil
}

func (a *API) handleError(w http.ResponseWriter, _ *http.Request, err error) {
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
	case errors.Is(err, execution.ErrDAGRunIDNotFound):
		code = api.ErrorCodeNotFound
		message = "dag-run not found"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(api.Error{
		Code:    code,
		Message: message,
	})
}

func (a *API) isAllowed(_ context.Context, perm config.Permission) error {
	if !a.config.Server.Permissions[perm] {
		return &Error{
			HTTPStatus: http.StatusForbidden,
			Code:       api.ErrorCodeForbidden,
			Message:    "Permission denied",
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
