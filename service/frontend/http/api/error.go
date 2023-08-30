package api

import (
	"github.com/samber/lo"
	"github.com/yohamta/dagu/service/frontend/models"
)

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
		Message:         lo.ToPtr(message),
		DetailedMessage: lo.ToPtr(detailedMessage),
	}
}

func NewInternalError(err error) *CodedError {
	return NewCodedError(500, NewAPIError("Internal Server Error", err.Error()))
}

func NewNotFoundError(err error) *CodedError {
	return NewCodedError(404, NewAPIError("Not Found", err.Error()))
}
