package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/service/audit"
)

var (
	// ErrBaseConfigNotAvailable is returned when base config management is not configured.
	ErrBaseConfigNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Base configuration management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	// ErrFailedToLoadBaseConfig is returned when reading the base config fails.
	ErrFailedToLoadBaseConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to load base configuration",
		HTTPStatus: http.StatusInternalServerError,
	}

	// ErrFailedToSaveBaseConfig is returned when writing the base config fails.
	ErrFailedToSaveBaseConfig = &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    "Failed to save base configuration",
		HTTPStatus: http.StatusInternalServerError,
	}
)

// GetBaseConfig returns the current base DAG configuration YAML. Requires developer role or above.
func (a *API) GetBaseConfig(ctx context.Context, _ api.GetBaseConfigRequestObject) (api.GetBaseConfigResponseObject, error) {
	if err := a.requireBaseConfigManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDeveloperOrAbove(ctx); err != nil {
		return nil, err
	}

	yamlSpec, err := a.baseConfigStore.GetSpec(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load base config", tag.Error(err))
		return nil, ErrFailedToLoadBaseConfig
	}

	errs := validateBaseConfig(ctx, yamlSpec)

	return api.GetBaseConfig200JSONResponse{
		Spec:   yamlSpec,
		Errors: errs,
	}, nil
}

// UpdateBaseConfig validates and saves the base DAG configuration YAML. Requires developer role or above.
func (a *API) UpdateBaseConfig(ctx context.Context, request api.UpdateBaseConfigRequestObject) (api.UpdateBaseConfigResponseObject, error) {
	if err := a.requireBaseConfigManagement(); err != nil {
		return nil, err
	}
	if err := a.requireDeveloperOrAbove(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	// Validate before saving — reject on validation error.
	if errs := validateBaseConfig(ctx, request.Body.Spec); len(errs) > 0 {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    errs[0],
			HTTPStatus: http.StatusBadRequest,
		}
	}

	if err := a.baseConfigStore.UpdateSpec(ctx, []byte(request.Body.Spec)); err != nil {
		logger.Error(ctx, "Failed to save base config", tag.Error(err))
		return nil, ErrFailedToSaveBaseConfig
	}

	a.logAudit(ctx, audit.CategorySystem, "base_config_update", nil)

	return api.UpdateBaseConfig200JSONResponse{
		Errors: []string{},
	}, nil
}

func (a *API) requireBaseConfigManagement() error {
	if a.baseConfigStore == nil {
		return ErrBaseConfigNotAvailable
	}
	return nil
}

// validateBaseConfig parses the YAML spec and returns any validation errors.
// Returns nil when the spec is empty or valid.
func validateBaseConfig(ctx context.Context, yamlSpec string) []string {
	if yamlSpec == "" {
		return nil
	}

	_, err := spec.LoadYAML(ctx,
		[]byte(yamlSpec),
		spec.WithoutEval(),
	)

	var loadErrs core.ErrorList
	if errors.As(err, &loadErrs) {
		return loadErrs.ToStringList()
	}
	if err != nil {
		return []string{err.Error()}
	}

	return nil
}
