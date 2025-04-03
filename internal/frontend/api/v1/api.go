package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
)

var _ api.StrictServerInterface = (*API)(nil)

type API struct {
	client client.Client
}

func New(cli client.Client) *API {
	return &API{client: cli}
}

func (a *API) ConfigureRoutes(r chi.Router, url string) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to get swagger: %w", err)
	}

	swagger.Servers = openapi3.Servers{
		&openapi3.Server{URL: url},
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
		handler := api.NewStrictHandlerWithOptions(a, nil, options)
		r.Mount("/", api.Handler(handler))
	})

	return nil
}

func (a *API) handleError(w http.ResponseWriter, _ *http.Request, err error) {
	var apiErr *APIError
	switch err := err.(type) {
	case *APIError:
		apiErr = err
	case APIError:
		apiErr = &err
	}

	var (
		response = api.Error{
			Code:    "internal_error",
			Message: "An unexpected error occurred",
		}
		httpStatusCode = http.StatusInternalServerError
	)

	if apiErr != nil {
		response.Code = apiErr.Code
		response.Message = apiErr.Message
		httpStatusCode = apiErr.HTTPStatus
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(response)
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
