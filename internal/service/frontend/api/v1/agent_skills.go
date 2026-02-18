package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionSkillCreate  = "agent_skill_create"
	auditActionSkillUpdate  = "agent_skill_update"
	auditActionSkillDelete  = "agent_skill_delete"
	auditActionSkillsEnable = "agent_skills_set_enabled"
)

var (
	errAgentSkillStoreNotAvailable = &Error{
		Code:       api.ErrorCodeForbidden,
		Message:    "Agent skill management is not available",
		HTTPStatus: http.StatusForbidden,
	}

	errSkillNotFound = &Error{
		Code:       api.ErrorCodeNotFound,
		Message:    "Skill not found",
		HTTPStatus: http.StatusNotFound,
	}

	errSkillAlreadyExists = &Error{
		Code:       api.ErrorCodeAlreadyExists,
		Message:    "Skill already exists",
		HTTPStatus: http.StatusConflict,
	}
)

// ListAgentSkills returns all configured skills with enabled status. Requires admin role.
func (a *API) ListAgentSkills(ctx context.Context, _ api.ListAgentSkillsRequestObject) (api.ListAgentSkillsResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	skills, err := a.agentSkillStore.List(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list agent skills", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to list skills", HTTPStatus: http.StatusInternalServerError}
	}

	enabledSkills, err := a.loadEnabledSkills(ctx)
	if err != nil {
		return nil, err
	}

	skillResponses := make([]api.SkillResponse, 0, len(skills))
	for _, s := range skills {
		skillResponses = append(skillResponses, toSkillResponse(s, isSkillEnabled(enabledSkills, s.ID)))
	}

	return api.ListAgentSkills200JSONResponse{Skills: skillResponses}, nil
}

// CreateAgentSkill creates a new skill. Requires admin role.
func (a *API) CreateAgentSkill(ctx context.Context, request api.CreateAgentSkillRequestObject) (api.CreateAgentSkillResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
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
	if strings.TrimSpace(body.Knowledge) == "" {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "knowledge is required and cannot be empty",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	// Generate or validate ID
	id := valueOf(body.Id)
	if id == "" {
		existingIDs := a.collectSkillIDs(ctx)
		id = agent.UniqueID(body.Name, existingIDs, "skill")
	}
	if err := agent.ValidateSkillID(id); err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("invalid skill ID: %v", err),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	skill := &agent.Skill{
		ID:        id,
		Name:      body.Name,
		Knowledge: body.Knowledge,
		Type:      agent.SkillTypeCustom,
	}
	if body.Description != nil {
		skill.Description = *body.Description
	}
	if body.Version != nil {
		skill.Version = *body.Version
	}
	if body.Author != nil {
		skill.Author = *body.Author
	}
	if body.Tags != nil {
		skill.Tags = *body.Tags
	}

	if err := a.agentSkillStore.Create(ctx, skill); err != nil {
		if errors.Is(err, agent.ErrSkillAlreadyExists) || errors.Is(err, agent.ErrSkillNameAlreadyExists) {
			return nil, errSkillAlreadyExists
		}
		logger.Error(ctx, "Failed to create agent skill", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to create skill", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSkillCreate, map[string]any{
		"skill_id": id,
		"name":     body.Name,
	})

	return api.CreateAgentSkill201JSONResponse(toSkillResponse(skill, false)), nil
}

// GetAgentSkill returns a single skill by ID. Requires admin role.
func (a *API) GetAgentSkill(ctx context.Context, request api.GetAgentSkillRequestObject) (api.GetAgentSkillResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	skill, err := a.agentSkillStore.GetByID(ctx, request.SkillId)
	if err != nil {
		if errors.Is(err, agent.ErrSkillNotFound) {
			return nil, errSkillNotFound
		}
		if errors.Is(err, agent.ErrInvalidSkillID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid skill ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to get skill", HTTPStatus: http.StatusInternalServerError}
	}

	enabledSkills, err := a.loadEnabledSkills(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetAgentSkill200JSONResponse(toSkillResponse(skill, isSkillEnabled(enabledSkills, skill.ID))), nil
}

// UpdateAgentSkill updates an existing skill. Requires admin role.
func (a *API) UpdateAgentSkill(ctx context.Context, request api.UpdateAgentSkillRequestObject) (api.UpdateAgentSkillResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	existing, err := a.agentSkillStore.GetByID(ctx, request.SkillId)
	if err != nil {
		if errors.Is(err, agent.ErrSkillNotFound) {
			return nil, errSkillNotFound
		}
		if errors.Is(err, agent.ErrInvalidSkillID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid skill ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to get skill", HTTPStatus: http.StatusInternalServerError}
	}

	applySkillUpdates(existing, request.Body)

	if err := a.agentSkillStore.Update(ctx, existing); err != nil {
		if errors.Is(err, agent.ErrSkillAlreadyExists) || errors.Is(err, agent.ErrSkillNameAlreadyExists) {
			return nil, errSkillAlreadyExists
		}
		logger.Error(ctx, "Failed to update agent skill", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to update skill", HTTPStatus: http.StatusInternalServerError}
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSkillUpdate, map[string]any{
		"skill_id": request.SkillId,
	})

	enabledSkills, err := a.loadEnabledSkills(ctx)
	if err != nil {
		return nil, err
	}

	return api.UpdateAgentSkill200JSONResponse(toSkillResponse(existing, isSkillEnabled(enabledSkills, existing.ID))), nil
}

// DeleteAgentSkill removes a skill. Requires admin role.
func (a *API) DeleteAgentSkill(ctx context.Context, request api.DeleteAgentSkillRequestObject) (api.DeleteAgentSkillResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := a.agentSkillStore.Delete(ctx, request.SkillId); err != nil {
		if errors.Is(err, agent.ErrSkillNotFound) {
			return nil, errSkillNotFound
		}
		if errors.Is(err, agent.ErrInvalidSkillID) {
			return nil, &Error{Code: api.ErrorCodeBadRequest, Message: fmt.Sprintf("invalid skill ID: %v", err), HTTPStatus: http.StatusBadRequest}
		}
		logger.Error(ctx, "Failed to delete agent skill", tag.Error(err))
		return nil, &Error{Code: api.ErrorCodeInternalError, Message: "Failed to delete skill", HTTPStatus: http.StatusInternalServerError}
	}

	// Remove from enabled skills if present (best-effort).
	if err := a.removeFromEnabledSkills(ctx, request.SkillId); err != nil {
		logger.Warn(ctx, "Failed to remove skill from enabled list after deletion",
			tag.Error(err), tag.String("skill_id", request.SkillId))
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSkillDelete, map[string]any{
		"skill_id": request.SkillId,
	})

	return api.DeleteAgentSkill204Response{}, nil
}

// SetEnabledSkills sets the list of enabled skill IDs. Requires admin role.
func (a *API) SetEnabledSkills(ctx context.Context, request api.SetEnabledSkillsRequestObject) (api.SetEnabledSkillsResponseObject, error) {
	if err := a.requireSkillManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, ErrInvalidRequestBody
	}

	// Validate all skill IDs exist using a single List call.
	knownIDs := a.collectSkillIDs(ctx)
	for _, id := range request.Body.SkillIds {
		if _, exists := knownIDs[id]; !exists {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    fmt.Sprintf("skill not found: %s", id),
				HTTPStatus: http.StatusBadRequest,
			}
		}
	}

	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil || cfg == nil {
		return nil, ErrFailedToLoadAgentConfig
	}

	cfg.EnabledSkills = request.Body.SkillIds
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return nil, ErrFailedToSaveAgentConfig
	}

	a.logAudit(ctx, audit.CategoryAgent, auditActionSkillsEnable, map[string]any{
		"skill_ids": request.Body.SkillIds,
	})

	return api.SetEnabledSkills200JSONResponse{EnabledSkills: cfg.EnabledSkills}, nil
}

func (a *API) loadEnabledSkills(ctx context.Context) ([]string, error) {
	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		return nil, ErrFailedToLoadAgentConfig
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.EnabledSkills, nil
}

func (a *API) requireSkillManagement() error {
	if a.agentSkillStore == nil {
		return errAgentSkillStoreNotAvailable
	}
	if a.agentConfigStore == nil {
		return ErrAgentConfigNotAvailable
	}
	return nil
}

func toSkillResponse(skill *agent.Skill, enabled bool) api.SkillResponse {
	return api.SkillResponse{
		Id:          skill.ID,
		Name:        skill.Name,
		Description: ptrOf(skill.Description),
		Version:     ptrOf(skill.Version),
		Author:      ptrOf(skill.Author),
		Tags:        ptrOf(skill.Tags),
		Type:        api.SkillResponseType(skill.Type),
		Knowledge:   skill.Knowledge,
		Enabled:     enabled,
	}
}

func applySkillUpdates(skill *agent.Skill, update *api.UpdateSkillRequest) {
	if update.Name != nil && strings.TrimSpace(*update.Name) != "" {
		skill.Name = *update.Name
	}
	if update.Description != nil {
		skill.Description = *update.Description
	}
	if update.Version != nil {
		skill.Version = *update.Version
	}
	if update.Author != nil {
		skill.Author = *update.Author
	}
	if update.Tags != nil {
		skill.Tags = *update.Tags
	}
	if update.Knowledge != nil && strings.TrimSpace(*update.Knowledge) != "" {
		skill.Knowledge = *update.Knowledge
	}
}

func (a *API) collectSkillIDs(ctx context.Context) map[string]struct{} {
	skills, err := a.agentSkillStore.List(ctx)
	if err != nil {
		return make(map[string]struct{})
	}
	ids := make(map[string]struct{}, len(skills))
	for _, s := range skills {
		ids[s.ID] = struct{}{}
	}
	return ids
}

func isSkillEnabled(enabledSkills []string, skillID string) bool {
	return slices.Contains(enabledSkills, skillID)
}

func (a *API) removeFromEnabledSkills(ctx context.Context, skillID string) error {
	cfg, err := a.agentConfigStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || !slices.Contains(cfg.EnabledSkills, skillID) {
		return nil
	}

	cfg.EnabledSkills = slices.DeleteFunc(cfg.EnabledSkills, func(id string) bool {
		return id == skillID
	})
	if err := a.agentConfigStore.Save(ctx, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
