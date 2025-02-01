package dag

import (
	"github.com/dagu-org/dagu/internal/frontend/gen/models"
	"github.com/go-openapi/swag"
)

type codedError struct {
	Code     int
	APIError *models.APIError
}

func newInternalError(err error) *codedError {
	return &codedError{Code: 500, APIError: &models.APIError{
		Message:         swag.String("Internal Server Error"),
		DetailedMessage: swag.String(err.Error()),
	}}
}

func newNotFoundError(err error) *codedError {
	return &codedError{Code: 404, APIError: &models.APIError{
		Message:         swag.String("Not Found"),
		DetailedMessage: swag.String(err.Error()),
	}}
}

func newBadRequestError(err error) *codedError {
	return &codedError{Code: 400, APIError: &models.APIError{
		Message:         swag.String("Bad Request"),
		DetailedMessage: swag.String(err.Error()),
	}}
}
