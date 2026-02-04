package api

import (
	"fmt"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
)

func WriteErrorResponse(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*Error); ok {
		w.WriteHeader(apiErr.HTTPStatus)
		code := apiErr.Code
		message := apiErr.Message
		if message != "" {
			_, _ = fmt.Fprintf(w, `{"code": "%s", "message": "%s"}`, code, message)
		} else {
			_, _ = fmt.Fprintf(w, `{"code": "%s"}`, code)
		}
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, `{"code": "internal_server_error"}`)
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
