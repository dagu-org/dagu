package api

import "fmt"

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
