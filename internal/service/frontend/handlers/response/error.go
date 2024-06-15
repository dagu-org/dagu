package response

import (
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/models"
	"github.com/go-openapi/swag"
)

func NewErrorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

type CodedError struct {
	Code     int
	APIError *models.APIError
}

func NewCodedError(code int, apiError *models.APIError) *CodedError {
	return &CodedError{
		Code:     code,
		APIError: apiError,
	}
}

func NewAPIError(message, detailedMessage string) *models.APIError {
	return &models.APIError{
		Message:         swag.String(message),
		DetailedMessage: swag.String(detailedMessage),
	}
}

func NewInternalError(err error) *CodedError {
	return NewCodedError(500, NewAPIError("Internal Server Error", err.Error()))
}

func NewNotFoundError(err error) *CodedError {
	return NewCodedError(404, NewAPIError("Not Found", err.Error()))
}

func NewBadRequestError(err error) *CodedError {
	return NewCodedError(400, NewAPIError("Bad Request", err.Error()))
}
