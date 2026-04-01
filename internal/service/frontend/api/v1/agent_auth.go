// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agentoauth"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionAgentAuthStart      = "agent_auth_start_login"
	auditActionAgentAuthComplete   = "agent_auth_complete_login"
	auditActionAgentAuthDisconnect = "agent_auth_disconnect"
)

// ListAgentAuthProviders returns subscription-backed auth provider status. Requires admin role.
func (a *API) ListAgentAuthProviders(ctx context.Context, _ api.ListAgentAuthProvidersRequestObject) (api.ListAgentAuthProvidersResponseObject, error) {
	if err := a.requireAgentAuthManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	statuses, err := a.agentOAuthManager.Status(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list agent auth providers", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to list auth providers", HTTPStatus: http.StatusInternalServerError}
	}

	providers := make([]api.AgentAuthProviderStatus, 0, len(statuses))
	for _, status := range statuses {
		providers = append(providers, toAgentAuthProviderStatus(status))
	}

	return api.ListAgentAuthProviders200JSONResponse{Providers: providers}, nil
}

// StartAgentAuthProviderLogin starts a manual OAuth login flow. Requires admin role.
func (a *API) StartAgentAuthProviderLogin(ctx context.Context, request api.StartAgentAuthProviderLoginRequestObject) (api.StartAgentAuthProviderLoginResponseObject, error) {
	if err := a.requireAgentAuthManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	result, err := a.agentOAuthManager.StartLogin(ctx, request.ProviderId)
	if err != nil {
		return nil, toAgentAuthError(ctx, "start login", err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionAgentAuthStart, map[string]any{
		"provider": request.ProviderId,
	})

	return api.StartAgentAuthProviderLogin200JSONResponse{
		FlowId:       result.FlowID,
		AuthUrl:      result.AuthURL,
		Instructions: ptrOf(result.Instructions),
	}, nil
}

// CompleteAgentAuthProviderLogin completes a manual OAuth login flow. Requires admin role.
func (a *API) CompleteAgentAuthProviderLogin(ctx context.Context, request api.CompleteAgentAuthProviderLoginRequestObject) (api.CompleteAgentAuthProviderLoginResponseObject, error) {
	if err := a.requireAgentAuthManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	cred, err := a.agentOAuthManager.CompleteLogin(ctx, request.ProviderId, agentoauth.CompleteLoginInput{
		FlowID:      request.Body.FlowId,
		RedirectURL: valueOf(request.Body.RedirectUrl),
		Code:        valueOf(request.Body.Code),
	})
	if err != nil {
		return nil, toAgentAuthError(ctx, "complete login", err)
	}

	status, err := a.providerStatusFromCredential(ctx, request.ProviderId, cred)
	if err != nil {
		return nil, err
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionAgentAuthComplete, map[string]any{
		"provider": request.ProviderId,
	})

	return api.CompleteAgentAuthProviderLogin200JSONResponse{
		Provider: status,
	}, nil
}

func (a *API) providerStatusFromCredential(ctx context.Context, providerID string, cred *agentoauth.Credential) (api.AgentAuthProviderStatus, error) {
	status, err := a.agentOAuthManager.ProviderStatus(ctx, providerID, cred)
	if err != nil {
		if errors.Is(err, agentoauth.ErrUnsupportedProvider) {
			return api.AgentAuthProviderStatus{}, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Unsupported auth provider",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		logger.Error(ctx, "Failed to load agent auth provider status", tag.Error(err), slog.String("provider", providerID))
		return api.AgentAuthProviderStatus{}, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to load provider status",
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	return toAgentAuthProviderStatus(status), nil
}

// DisconnectAgentAuthProviderLogin deletes the stored OAuth credential. Requires admin role.
func (a *API) DisconnectAgentAuthProviderLogin(ctx context.Context, request api.DisconnectAgentAuthProviderLoginRequestObject) (api.DisconnectAgentAuthProviderLoginResponseObject, error) {
	if err := a.requireAgentAuthManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentOAuthManager.Logout(ctx, request.ProviderId); err != nil {
		return nil, toAgentAuthError(ctx, "disconnect provider", err)
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionAgentAuthDisconnect, map[string]any{
		"provider": request.ProviderId,
	})

	return api.DisconnectAgentAuthProviderLogin204Response{}, nil
}

func (a *API) providerStatus(ctx context.Context, providerID string) (api.AgentAuthProviderStatus, error) {
	status, err := a.agentOAuthManager.ProviderStatus(ctx, providerID, nil)
	if err != nil {
		if errors.Is(err, agentoauth.ErrUnsupportedProvider) {
			return api.AgentAuthProviderStatus{}, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Unsupported auth provider",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		logger.Error(ctx, "Failed to load agent auth provider status", tag.Error(err), slog.String("provider", providerID))
		return api.AgentAuthProviderStatus{}, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to load provider status",
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	return toAgentAuthProviderStatus(status), nil
}

func toAgentAuthProviderStatus(status agentoauth.ProviderStatus) api.AgentAuthProviderStatus {
	var accountID *string
	if status.AccountID != "" {
		accountID = ptrOf(status.AccountID)
	}
	return api.AgentAuthProviderStatus{
		Id:         status.ID,
		Name:       status.Name,
		Connected:  status.Connected,
		ExpiresAt:  status.ExpiresAt,
		CanRefresh: ptrOf(status.CanRefresh),
		AccountId:  accountID,
	}
}

func toAgentAuthError(ctx context.Context, action string, err error) error {
	switch {
	case errors.Is(err, agentoauth.ErrUnsupportedProvider):
		return &Error{Code: api.ErrorCodeBadRequest, Message: "Unsupported auth provider", HTTPStatus: http.StatusBadRequest}
	case errors.Is(err, agentoauth.ErrFlowNotFound):
		return &Error{Code: api.ErrorCodeBadRequest, Message: "OAuth login flow was not found or has already been used", HTTPStatus: http.StatusBadRequest}
	case errors.Is(err, agentoauth.ErrFlowExpired):
		return &Error{Code: api.ErrorCodeBadRequest, Message: "OAuth login flow expired; start a new login", HTTPStatus: http.StatusBadRequest}
	case errors.Is(err, agentoauth.ErrStateMismatch):
		return &Error{Code: api.ErrorCodeBadRequest, Message: "OAuth state mismatch", HTTPStatus: http.StatusBadRequest}
	default:
		logger.Error(ctx, "Failed to manage agent auth provider", slog.String("action", action), tag.Error(err))
		return &Error{Code: api.ErrorCodeInternalError, Message: "Failed to manage agent auth provider", HTTPStatus: http.StatusInternalServerError}
	}
}
