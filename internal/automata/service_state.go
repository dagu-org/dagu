// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/core/exec"
)

func (s *Service) statePath(name string) string {
	return filepath.Join(s.stateDir, name, "state.json")
}

func (s *Service) loadState(_ context.Context, name string) (*State, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	path := s.statePath(name)
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	return &state, nil
}

func (s *Service) saveState(_ context.Context, name string, state *State) error {
	if err := validateName(name); err != nil {
		return err
	}
	if state == nil {
		return errors.New("state is required")
	}
	state.LastUpdatedAt = s.clock()
	path := s.statePath(name)
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	return fileutil.WriteJSONAtomic(path, state, stateFilePerm)
}

func (s *Service) ResetState(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return err
	}
	if err := s.cleanupRuntime(ctx, name, true); err != nil {
		return err
	}
	state, err := s.loadState(ctx, name)
	if err != nil {
		return err
	}
	if state == nil {
		state = newInitialState()
	} else {
		resetRuntimeState(state)
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) resetRuntimeForSpecChange(ctx context.Context, name string) error {
	state, err := s.loadState(ctx, name)
	if err != nil || state == nil {
		return err
	}
	if err := s.cleanupRuntime(ctx, name, true); err != nil {
		return err
	}
	state.SessionID = ""
	state.CurrentRunRef = nil
	state.LastRunRef = nil
	state.CurrentCycleID = ""
	state.PendingPrompt = nil
	state.PendingResponse = nil
	state.WaitingReason = WaitingReasonNone
	state.LastSummary = ""
	state.LastError = ""
	if (state.State == StateRunning || state.State == StateWaiting) &&
		len(state.PendingTurnMessages) == 0 {
		queueTurnMessage(state, "config_updated",
			"Automata configuration changed. Continue with the latest configuration and current instruction.",
			s.clock(),
		)
		state.State = StateRunning
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) ensureState(ctx context.Context, def *Definition) (*State, error) {
	state, err := s.loadState(ctx, def.Name)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = newInitialState()
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	changed, err := normalizePersistedTasks(&state.Tasks)
	if err != nil {
		return nil, err
	}
	templateChanged, err := normalizePersistedTaskTemplates(&state.TaskTemplates)
	if err != nil {
		return nil, err
	}
	changed = changed || templateChanged
	if state.State == "" {
		state.State = StateIdle
		changed = true
	}
	if state.TaskTemplates == nil {
		state.TaskTemplates = []TaskTemplate{}
		changed = true
	}
	if state.Tasks == nil {
		state.Tasks = []Task{}
		changed = true
	}
	if len(state.TaskTemplates) == 0 && len(state.Tasks) > 0 {
		state.TaskTemplates = cloneTaskTemplatesFromTasks(state.Tasks)
		changed = true
	}
	if isService(def) {
		if state.State == StateFinished {
			state.State = StateIdle
			state.FinishedAt = time.Time{}
			changed = true
		}
		if state.ActivatedAt.IsZero() &&
			(state.State == StateRunning || state.State == StateWaiting || state.State == StatePaused) {
			state.ActivatedAt = firstNonZeroTime(state.StartRequestedAt, state.LastUpdatedAt, s.clock())
			changed = true
		}
	} else if !state.ActivatedAt.IsZero() || state.ActivatedBy != "" {
		state.ActivatedAt = time.Time{}
		state.ActivatedBy = ""
		changed = true
	}
	if changed {
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (s *Service) cleanupRuntime(ctx context.Context, name string, deleteSession bool) error {
	state, err := s.loadState(ctx, name)
	if err != nil || state == nil {
		return err
	}
	if err := s.cancelTrackedChildRun(ctx, state.CurrentRunRef); err != nil {
		return err
	}
	if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
		return err
	}
	if deleteSession && state.SessionID != "" && s.sessionStore != nil {
		if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) cancelTrackedChildRun(ctx context.Context, ref *exec.DAGRunRef) error {
	if ref == nil {
		return nil
	}
	status, err := s.lookupRunStatus(ctx, ref)
	if err != nil {
		return err
	}
	if status == nil {
		return nil
	}
	if !status.Status.IsActive() && !status.Status.IsWaiting() {
		return nil
	}
	return s.requestChildRunCancel(ctx, ref)
}

func (s *Service) cancelAutomataSession(ctx context.Context, name, sessionID string) error {
	if sessionID == "" || s.agentAPI == nil {
		return nil
	}
	err := s.agentAPI.CancelSession(ctx, sessionID, s.systemUser(name).UserID)
	if err == nil || errors.Is(err, agent.ErrSessionNotFound) {
		return nil
	}
	return err
}

func (s *Service) reassignSessionUser(ctx context.Context, sessionID, newName string) error {
	if sessionID == "" || s.sessionStore == nil {
		return nil
	}
	if reassigner, ok := s.sessionStore.(sessionUserReassigner); ok {
		return reassigner.ReassignSessionUser(ctx, sessionID, s.systemUser(newName).UserID)
	}
	return errors.New("session store does not support automata rename with transcript preservation")
}

func (s *Service) assertAutomataTargetAvailable(name string) error {
	if _, err := os.Stat(filepath.Clean(s.definitionPath(name))); err == nil {
		return fmt.Errorf("automata %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, name)); err == nil {
		return fmt.Errorf("automata %q already has existing runtime state", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) systemUser(name string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   "__automata__:" + name,
		Username: "automata/" + name,
		Role:     auth.RoleAdmin,
	}
}

func resetRuntimeState(state *State) {
	if state == nil {
		return
	}
	clearCurrentCycleState(state)
	state.ActivatedAt = time.Time{}
	state.ActivatedBy = ""
	state.LastScheduleMinute = time.Time{}
}

func clearCurrentCycleState(state *State) {
	if state == nil {
		return
	}
	state.WaitingReason = WaitingReasonNone
	state.PendingPrompt = nil
	state.PendingResponse = nil
	state.PendingTurnMessages = nil
	state.SessionID = ""
	state.CurrentRunRef = nil
	state.LastRunRef = nil
	state.CurrentCycleID = ""
	state.Tasks = []Task{}
	state.StartRequestedAt = time.Time{}
	state.PausedAt = time.Time{}
	state.PausedBy = ""
	state.PausedFromState = ""
	state.FinishedAt = time.Time{}
	state.LastSummary = ""
	state.LastError = ""
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func (s *Service) requestChildRunCancel(ctx context.Context, ref *exec.DAGRunRef) error {
	if ref == nil {
		return nil
	}
	status, _ := s.lookupRunStatus(ctx, ref)
	if status != nil && status.WorkerID != "" && s.coordinatorCli != nil {
		return s.coordinatorCli.RequestCancel(ctx, ref.Name, ref.ID, nil)
	}
	if s.dagRunCtrl != nil {
		dag, err := s.dagStore.GetDetails(ctx, ref.Name)
		if err != nil {
			return err
		}
		return s.dagRunCtrl.Stop(ctx, dag, ref.ID)
	}
	attempt, err := s.dagRunStore.FindAttempt(ctx, *ref)
	if err == nil {
		return attempt.Abort(ctx)
	}
	return errors.New("dag run control is not configured")
}

func (s *Service) lookupRunStatus(ctx context.Context, ref *exec.DAGRunRef) (*exec.DAGRunStatus, error) {
	if ref == nil {
		return nil, nil
	}
	statuses, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithFrom(exec.NewUTC(time.Unix(0, 0))),
		exec.WithExactName(ref.Name),
		exec.WithDAGRunID(ref.ID),
		exec.WithLimit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	return statuses[0], nil
}
