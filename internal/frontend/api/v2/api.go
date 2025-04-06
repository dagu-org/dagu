package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/auth"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
)

var _ api.StrictServerInterface = (*API)(nil)

type API struct {
	client             client.Client
	remoteNodes        map[string]config.RemoteNode
	apiBasePath        string
	logEncodingCharset string
	config             *config.Config
}

func New(
	cli client.Client,
	cfg *config.Config,
) *API {
	remoteNodes := make(map[string]config.RemoteNode)
	for _, n := range cfg.Server.RemoteNodes {
		remoteNodes[n.Name] = n
	}

	return &API{
		client:             cli,
		logEncodingCharset: cfg.UI.LogEncodingCharset,
		remoteNodes:        remoteNodes,
		apiBasePath:        cfg.Server.APIBasePath,
		config:             cfg,
	}
}

func (a *API) ConfigureRoutes(r chi.Router, baseURL string) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to get swagger: %w", err)
	}

	// Parse the baseURL to extract components
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Create a list of server URLs
	serverURLs := []string{baseURL}

	// Add localhost as an alternative if the host is 127.0.0.1
	host := parsedURL.Hostname()
	if host == "127.0.0.1" {
		// Create a new URL with localhost instead of 127.0.0.1
		localhostURL := *parsedURL
		localhostURL.Host = fmt.Sprintf("localhost:%s", parsedURL.Port())
		serverURLs = append(serverURLs, localhostURL.String())
	}

	// Add 127.0.0.1 as an alternative if the host is localhost
	if host == "localhost" {
		// Create a new URL with 127.0.0.1 instead of localhost
		ipURL := *parsedURL
		ipURL.Host = fmt.Sprintf("127.0.0.1:%s", parsedURL.Port())
		serverURLs = append(serverURLs, ipURL.String())
	}

	// Set the server URLs in the swagger spec
	swagger.Servers = make(openapi3.Servers, 0, len(serverURLs))
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
		authOptions := auth.Options{
			Realm:            "restricted",
			APITokenEnabled:  a.config.Server.Auth.Token.Enabled,
			APIToken:         a.config.Server.Auth.Token.Value,
			BasicAuthEnabled: a.config.Server.Auth.Basic.Enabled,
			Creds: map[string]string{
				a.config.Server.Auth.Basic.Username: a.config.Server.Auth.Basic.Password,
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
	case errors.Is(err, persistence.ErrRequestIDNotFound):
		code = api.ErrorCodeNotFound
		message = "Request ID not found"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(api.Error{
		Code:    code,
		Message: message,
	})
}

func ptr[T any](v T) *T {
	if reflect.ValueOf(v).IsZero() {
		return nil
	}

	return &v
}

func value[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}
