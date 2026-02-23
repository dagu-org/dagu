package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
)

// ActivateLicense handles license activation from the frontend.
func (a *API) ActivateLicense(ctx context.Context, request api.ActivateLicenseRequestObject) (api.ActivateLicenseResponseObject, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if a.licenseManager == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "License management is not available",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	if request.Body == nil || request.Body.Key == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "License key is required",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	result, err := a.licenseManager.ActivateWithKey(ctx, request.Body.Key)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	var expiry string
	if !result.Expiry.IsZero() {
		expiry = result.Expiry.Format("2006-01-02T15:04:05Z")
	}

	return api.ActivateLicense200JSONResponse{
		Plan:     &result.Plan,
		Features: &result.Features,
		Expiry:   &expiry,
	}, nil
}
