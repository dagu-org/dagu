package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
)

var _ api.StrictServerInterface = (*API)(nil)

type API struct {
}

func New() *API {
	return &API{}
}

func (a *API) ConfigureRoutes(r chi.Router) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to get swagger: %w", err)
	}
	// Override the server settings in the swagger spec. For more details, see:
	// https://github.com/oapi-codegen/oapi-codegen/issues/1123
	swagger.Servers = openapi3.Servers{&openapi3.Server{URL: "/api/v1"}}

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
		ResponseErrorHandlerFunc: a.errorHandler,
	}

	r.Group(func(r chi.Router) {
		handler := api.NewStrictHandlerWithOptions(a, nil, options)
		r.Mount("/", api.Handler(handler))
	})

	return nil
}

func (a *API) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
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

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	panic("unimplemented")
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	panic("unimplemented")
}

// GetDAGDetails implements api.StrictServerInterface.
func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	panic("unimplemented")
}

// ListDAGs implements api.StrictServerInterface.
func (a *API) ListDAGs(ctx context.Context, request api.ListDAGsRequestObject) (api.ListDAGsResponseObject, error) {
	panic("unimplemented")
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, request api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	panic("unimplemented")
}

// PostDAGAction implements api.StrictServerInterface.
func (a *API) PostDAGAction(ctx context.Context, request api.PostDAGActionRequestObject) (api.PostDAGActionResponseObject, error) {
	panic("unimplemented")
}

// SearchDAGs implements api.StrictServerInterface.
func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	panic("unimplemented")
}

func toPtr[T any](v T) *T {
	return &v
}
