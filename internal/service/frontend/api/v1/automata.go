// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	apiv1 "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/automata"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/audit"
	"github.com/go-chi/chi/v5"
)

func (a *API) configureAutomataRoutes(r chi.Router) {
	if a.automataService == nil {
		return
	}
	r.Route("/automata", func(r chi.Router) {
		r.Get("/", a.handleListAutomata)
		r.Get("/{name}", a.handleGetAutomata)
		r.Get("/{name}/spec", a.handleGetAutomataSpec)
		r.Put("/{name}/spec", a.handlePutAutomataSpec)
		r.Delete("/{name}", a.handleDeleteAutomata)
		r.Post("/{name}/start", a.handleStartAutomata)
		r.Post("/{name}/pause", a.handlePauseAutomata)
		r.Post("/{name}/resume", a.handleResumeAutomata)
		r.Post("/{name}/message", a.handleMessageAutomata)
		r.Post("/{name}/stage", a.handleOverrideAutomataStage)
		r.Post("/{name}/response", a.handleRespondAutomata)
	})
}

func (a *API) handleListAutomata(w http.ResponseWriter, r *http.Request) {
	items, err := a.automataService.List(r.Context())
	if err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusInternalServerError, apiv1.ErrorCodeInternalError, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"automata": items})
}

func (a *API) handleGetAutomata(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	item, err := a.automataService.Detail(r.Context(), name)
	if err != nil {
		writeAutomataError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) handleGetAutomataSpec(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	spec, err := a.automataService.GetSpec(r.Context(), name)
	if err != nil {
		writeAutomataError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spec": spec})
}

func (a *API) handlePutAutomataSpec(w http.ResponseWriter, r *http.Request) {
	if err := a.requireDAGWrite(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	var body struct {
		Spec string `json:"spec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	if err := a.automataService.PutSpec(r.Context(), name, body.Spec); err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "spec_upsert", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleDeleteAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireDAGWrite(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	if err := a.automataService.Delete(r.Context(), name); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "delete", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleStartAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	var body automata.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	userName := ""
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userName = user.Username
	}
	body.RequestedBy = userName
	if err := a.automataService.RequestStart(r.Context(), name, body); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "start", map[string]any{"name": name, "instruction": body.Instruction})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handlePauseAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	userName := ""
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userName = user.Username
	}
	if err := a.automataService.Pause(r.Context(), name, userName); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "pause", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleResumeAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	userName := ""
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userName = user.Username
	}
	if err := a.automataService.Resume(r.Context(), name, userName); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "resume", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleOverrideAutomataStage(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	var body automata.StageOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		body.RequestedBy = user.Username
	}
	if err := a.automataService.OverrideStage(r.Context(), name, body); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "stage_override", map[string]any{"name": name, "stage": body.Stage})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleMessageAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	var body automata.OperatorMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		body.RequestedBy = user.Username
	}
	if err := a.automataService.SubmitOperatorMessage(r.Context(), name, body); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "message", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleRespondAutomata(w http.ResponseWriter, r *http.Request) {
	if err := a.requireExecute(r.Context()); err != nil {
		WriteErrorResponse(w, err)
		return
	}
	name := chi.URLParam(r, "name")
	var body automata.HumanResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
		return
	}
	if err := a.automataService.SubmitHumanResponse(r.Context(), name, body); err != nil {
		writeAutomataError(w, err)
		return
	}
	a.logAudit(r.Context(), audit.CategoryAutomata, "respond", map[string]any{"name": name, "prompt_id": body.PromptID})
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAutomataError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exec.ErrDAGNotFound):
		WriteErrorResponse(w, NewAPIError(http.StatusNotFound, apiv1.ErrorCodeNotFound, err))
	case errors.Is(err, os.ErrNotExist):
		WriteErrorResponse(w, NewAPIError(http.StatusNotFound, apiv1.ErrorCodeNotFound, err))
	default:
		WriteErrorResponse(w, NewAPIError(http.StatusBadRequest, apiv1.ErrorCodeBadRequest, err))
	}
}
