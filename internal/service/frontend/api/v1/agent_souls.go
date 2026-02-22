package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionSoulCreate = "agent_soul_create"
	auditActionSoulUpdate = "agent_soul_update"
	auditActionSoulDelete = "agent_soul_delete"
)

var (
	errAgentSoulStoreNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent soul management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errSoulNotFound = &Error{
		Code:       api.ErrorCodeNotFound,
		Message:    "Soul not found",
		HTTPStatus: http.StatusNotFound,
	}

	errSoulAlreadyExists = &Error{
		Code:       api.ErrorCodeAlreadyExists,
		Message:    "Soul already exists",
		HTTPStatus: http.StatusConflict,
	}
)

// ListAgentSouls returns paginated souls with optional search. Requires admin role.
func (a *API) ListAgentSouls(ctx context.Context, request api.ListAgentSoulsRequestObject) (api.ListAgentSoulsResponseObject, error) {
	if err := a.requireSoulManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	pg := exec.NewPaginator(valueOf(request.Params.Page), valueOf(request.Params.PerPage))
	tags := parseCommaSeparatedTags(request.Params.Tags)

	result, err := a.agentSoulStore.Search(ctx, agent.SearchSoulsOptions{
		Paginator: pg,
		Query:     valueOf(request.Params.Q),
		Tags:      tags,
	})
	if err != nil {
		logger.Error(ctx, "Failed to search agent souls", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to list souls", HTTPStatus: http.StatusInternalServerError}
	}

	soulResponses := make([]api.SoulResponse, 0, len(result.Items))
	for _, m := range result.Items {
		soulResponses = append(soulResponses, toSoulMetadataResponse(m))
	}

	return api.ListAgentSouls200JSONResponse{
		Souls:      soulResponses,
		Pagination: toPagination(*result),
	}, nil
}

// CreateAgentSoul creates a new soul. Requires admin role.
func (a *API) CreateAgentSoul(ctx context.Context, request api.CreateAgentSoulRequestObject) (api.CreateAgentSoulResponseObject, error) {
	if err := a.requireSoulManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	body := request.Body

	if strings.TrimSpace(body.Name) == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "name is required and cannot be empty",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if strings.TrimSpace(body.Content) == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "content is required and cannot be empty",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	// Generate or validate ID.
	id := valueOf(body.Id)
	if id == "" {
		existingIDs := a.collectSoulIDs(ctx)
		id = agent.UniqueID(body.Name, existingIDs, "soul")
	}
	if err := agent.ValidateSoulID(id); err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid soul ID: %v", err),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	soul := &agent.Soul{
		ID:      id,
		Name:    body.Name,
		Content: body.Content,
	}
	if body.Description != nil {
		soul.Description = *body.Description
	}
	if body.Version != nil {
		soul.Version = *body.Version
	}
	if body.Author != nil {
		soul.Author = *body.Author
	}
	if body.Tags != nil {
		soul.Tags = *body.Tags
	}

	if err := a.agentSoulStore.Create(ctx, soul); err != nil {
		if errors.Is(err, agent.ErrSoulAlreadyExists) || errors.Is(err, agent.ErrSoulNameAlreadyExists) {
			return nil, errSoulAlreadyExists
		}
		logger.Error(ctx, "Failed to create agent soul", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to create soul", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSoulCreate, map[string]any{
		"soul_id": id,
		"name":    body.Name,
	})

	return api.CreateAgentSoul201JSONResponse(toSoulResponse(soul)), nil
}

// GetAgentSoul returns a single soul by ID. Requires admin role.
func (a *API) GetAgentSoul(ctx context.Context, request api.GetAgentSoulRequestObject) (api.GetAgentSoulResponseObject, error) {
	if err := a.requireSoulManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	soul, err := a.agentSoulStore.GetByID(ctx, request.SoulId)
	if err != nil {
		if errors.Is(err, agent.ErrSoulNotFound) {
			return nil, errSoulNotFound
		}
		if errors.Is(err, agent.ErrInvalidSoulID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid soul ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to get soul", HTTPStatus: http.StatusInternalServerError}
	}

	return api.GetAgentSoul200JSONResponse(toSoulResponse(soul)), nil
}

// UpdateAgentSoul updates an existing soul. Requires admin role.
func (a *API) UpdateAgentSoul(ctx context.Context, request api.UpdateAgentSoulRequestObject) (api.UpdateAgentSoulResponseObject, error) {
	if err := a.requireSoulManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	existing, err := a.agentSoulStore.GetByID(ctx, request.SoulId)
	if err != nil {
		if errors.Is(err, agent.ErrSoulNotFound) {
			return nil, errSoulNotFound
		}
		if errors.Is(err, agent.ErrInvalidSoulID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid soul ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to get soul", HTTPStatus: http.StatusInternalServerError}
	}

	applySoulUpdates(existing, request.Body)

	if err := a.agentSoulStore.Update(ctx, existing); err != nil {
		if errors.Is(err, agent.ErrSoulAlreadyExists) || errors.Is(err, agent.ErrSoulNameAlreadyExists) {
			return nil, errSoulAlreadyExists
		}
		logger.Error(ctx, "Failed to update agent soul", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to update soul", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSoulUpdate, map[string]any{
		"soul_id": request.SoulId,
	})

	return api.UpdateAgentSoul200JSONResponse(toSoulResponse(existing)), nil
}

// DeleteAgentSoul removes a soul. Requires admin role.
func (a *API) DeleteAgentSoul(ctx context.Context, request api.DeleteAgentSoulRequestObject) (api.DeleteAgentSoulResponseObject, error) {
	if err := a.requireSoulManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentSoulStore.Delete(ctx, request.SoulId); err != nil {
		if errors.Is(err, agent.ErrSoulNotFound) {
			return nil, errSoulNotFound
		}
		if errors.Is(err, agent.ErrInvalidSoulID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid soul ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		logger.Error(ctx, "Failed to delete agent soul", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to delete soul", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSoulDelete, map[string]any{
		"soul_id": request.SoulId,
	})

	return api.DeleteAgentSoul204Response{}, nil
}

func (a *API) requireSoulManagement() error {
	if a.agentSoulStore == nil {
		return errAgentSoulStoreNotAvailable
	}
	return nil
}

func toSoulResponse(soul *agent.Soul) api.SoulResponse {
	return api.SoulResponse{
		Id:          soul.ID,
		Name:        soul.Name,
		Description: ptrOf(soul.Description),
		Version:     ptrOf(soul.Version),
		Author:      ptrOf(soul.Author),
		Tags:        ptrOf(soul.Tags),
		Content:     ptrOf(soul.Content),
	}
}

func toSoulMetadataResponse(m agent.SoulMetadata) api.SoulResponse {
	return api.SoulResponse{
		Id:          m.ID,
		Name:        m.Name,
		Description: ptrOf(m.Description),
		Version:     ptrOf(m.Version),
		Author:      ptrOf(m.Author),
		Tags:        ptrOf(m.Tags),
	}
}

func applySoulUpdates(soul *agent.Soul, update *api.UpdateSoulRequest) {
	if update.Name != nil && strings.TrimSpace(*update.Name) != "" {
		soul.Name = *update.Name
	}
	if update.Description != nil {
		soul.Description = *update.Description
	}
	if update.Version != nil {
		soul.Version = *update.Version
	}
	if update.Author != nil {
		soul.Author = *update.Author
	}
	if update.Tags != nil {
		soul.Tags = *update.Tags
	}
	if update.Content != nil && strings.TrimSpace(*update.Content) != "" {
		soul.Content = *update.Content
	}
}

func (a *API) collectSoulIDs(ctx context.Context) map[string]struct{} {
	souls, err := a.agentSoulStore.List(ctx)
	if err != nil {
		return make(map[string]struct{})
	}
	ids := make(map[string]struct{}, len(souls))
	for _, s := range souls {
		ids[s.ID] = struct{}{}
	}
	return ids
}
