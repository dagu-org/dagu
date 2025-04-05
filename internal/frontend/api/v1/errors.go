package api

import (
	"fmt"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
)

func WriteErrorResponse(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*Error); ok {
		w.WriteHeader(apiErr.HTTPStatus)
		if apiErr.Message != "" {
			fmt.Fprintf(w, `{"error": "%s"}`, apiErr.Message)
		} else {
			fmt.Fprintf(w, `{"error": "%s"}`, apiErr.Code)
		}
		return
	}

	apiErr := newInternalError(err)
	w.WriteHeader(apiErr.HTTPStatus)
	fmt.Fprintf(w, `{"error": "%s"}`, apiErr.Message)
}

// Error is an error that has an associated HTTP status code.
type Error struct {
	// Code is the error code to return.
	Code api.ErrorCode
	// HTTPStatus is the HTTP status code to return.
	HTTPStatus int
	// Message is the error message to return.
	Message string
}

// Error returns the error message.
func (e Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewAPIError(httpCode int, code api.ErrorCode, err error) *Error {
	apiErr := &Error{
		Code:       code,
		HTTPStatus: httpCode,
	}
	if err != nil {
		apiErr.Message = err.Error()
	}
	return apiErr
}

func newInternalError(err error) *Error {
	return &Error{
		Code:       api.ErrorCodeInternalError,
		HTTPStatus: 500,
		Message:    "An internal error occurred",
	}
}

func newNotFoundError(code api.ErrorCode, err error) *Error {
	apiErr := &Error{
		Code:       "not_found",
		HTTPStatus: 404,
	}
	if err != nil {
		apiErr.Message = err.Error()
	}
	return apiErr
}

func newBadRequestError(code api.ErrorCode, err error) *Error {
	apiErr := &Error{
		Code:       code,
		HTTPStatus: 400,
	}
	if err != nil {
		apiErr.Message = err.Error()
	}
	return apiErr
}
