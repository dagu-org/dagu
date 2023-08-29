package api

import (
	"github.com/yohamta/dagu/service/frontend/models"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
)

func Configure(api *operations.DaguAPI) {
	registerWorkflows(api)
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
		Message:         message,
		DetailedMessage: detailedMessage,
	}
}

func NewInternalError(err error) *CodedError {
	return NewCodedError(500, NewAPIError("Internal Server Error", err.Error()))
}
