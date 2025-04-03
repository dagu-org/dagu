package api

import (
	"fmt"
)

// APIError is an error that has an associated HTTP status code.
type APIError struct {
	// Code is the error code to return.
	Code string
	// HTTPStatus is the HTTP status code to return.
	HTTPStatus int
	// Message is the error message to return.
	Message string
}

// Error returns the error message.
func (e APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newInternalError(err error) *APIError {
	return &APIError{
		Code:       "internal_error",
		HTTPStatus: 500,
		Message:    "An internal error occurred",
	}
}

func newNotFoundError(err error) *APIError {
	return &APIError{
		Code:       "not_found",
		HTTPStatus: 404,
		Message:    err.Error(),
	}
}

func newBadRequestError(err error) *APIError {
	return &APIError{
		Code:       "validation_error",
		HTTPStatus: 400,
		Message:    err.Error(),
	}
}

func newError(httpCode int, code string, message *string) *APIError {
	return &APIError{
		Code:       code,
		HTTPStatus: httpCode,
		Message:    *message,
	}
}
