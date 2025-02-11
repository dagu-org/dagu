package handlers

import (
	"github.com/dagu-org/dagu/internal/frontend/gen/models"
	"github.com/go-openapi/swag"
)

type codedError struct {
	HTTPCode int
	Code     string
	APIError *models.Error
}

func newInternalError(err error) *codedError {
	return &codedError{HTTPCode: 500, APIError: &models.Error{
		Code:    swag.String(models.ErrorCodeInternalError),
		Message: swag.String(err.Error()),
	}}
}

func newNotFoundError(err error) *codedError {
	return &codedError{HTTPCode: 404, APIError: &models.Error{
		Code:    swag.String(models.ErrorCodeNotFound),
		Message: swag.String(err.Error()),
	}}
}

func newBadRequestError(err error) *codedError {
	return &codedError{HTTPCode: 400, APIError: &models.Error{
		Code:    swag.String(models.ErrorCodeValidationError),
		Message: swag.String(err.Error()),
	}}
}

func newError(httpCode int, code string, message *string) *codedError {
	return &codedError{HTTPCode: httpCode, APIError: &models.Error{
		Code:    swag.String(code),
		Message: message,
	}}
}
