// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"net/http"

	"github.com/dagucloud/dagu/api/v1"
)

// Stub implementations for removed skill endpoints.
// These satisfy the StrictServerInterface but return 410 Gone.

var errSkillsRemoved = &Error{
	Code:       api.ErrorCodeNotFound,
	Message:    "Skill management has been removed",
	HTTPStatus: http.StatusGone,
}

func (a *API) ListAgentSkills(_ context.Context, _ api.ListAgentSkillsRequestObject) (api.ListAgentSkillsResponseObject, error) {
	return nil, errSkillsRemoved
}

func (a *API) CreateAgentSkill(_ context.Context, _ api.CreateAgentSkillRequestObject) (api.CreateAgentSkillResponseObject, error) {
	return nil, errSkillsRemoved
}

func (a *API) GetAgentSkill(_ context.Context, _ api.GetAgentSkillRequestObject) (api.GetAgentSkillResponseObject, error) {
	return nil, errSkillsRemoved
}

func (a *API) UpdateAgentSkill(_ context.Context, _ api.UpdateAgentSkillRequestObject) (api.UpdateAgentSkillResponseObject, error) {
	return nil, errSkillsRemoved
}

func (a *API) DeleteAgentSkill(_ context.Context, _ api.DeleteAgentSkillRequestObject) (api.DeleteAgentSkillResponseObject, error) {
	return nil, errSkillsRemoved
}

func (a *API) SetEnabledSkills(_ context.Context, _ api.SetEnabledSkillsRequestObject) (api.SetEnabledSkillsResponseObject, error) {
	return nil, errSkillsRemoved
}
