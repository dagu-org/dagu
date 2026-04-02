// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import "fmt"

type ViewStatus struct {
	DisplayStatus DisplayStatus
	Busy          bool
	NeedsInput    bool
}

func validateAutomataKind(kind AutomataKind) error {
	switch kind {
	case AutomataKindWorkflow, AutomataKindService:
		return nil
	default:
		return fmt.Errorf("invalid automata kind %q", kind)
	}
}

func isService(def *Definition) bool {
	return def != nil && normalizeAutomataKind(def.Kind) == AutomataKindService
}

func isServiceActivated(state *State) bool {
	if state == nil {
		return false
	}
	if !state.ActivatedAt.IsZero() {
		return true
	}
	return state.State == StateRunning || state.State == StateWaiting || state.State == StatePaused
}

func isBusyState(state *State) bool {
	if state == nil {
		return false
	}
	if state.CurrentRunRef != nil {
		return true
	}
	if len(state.PendingTurnMessages) > 0 {
		return true
	}
	return state.State == StateRunning
}

func DeriveView(def *Definition, state *State) ViewStatus {
	view := ViewStatus{
		DisplayStatus: DisplayStatusIdle,
		Busy:          isBusyState(state),
		NeedsInput:    state != nil && state.PendingPrompt != nil,
	}
	if state == nil {
		return view
	}

	if isService(def) {
		switch {
		case state.State == StatePaused:
			view.DisplayStatus = DisplayStatusPaused
		case isServiceActivated(state):
			view.DisplayStatus = DisplayStatusRunning
		default:
			view.DisplayStatus = DisplayStatusIdle
		}
		return view
	}

	switch state.State {
	case StateIdle:
		view.DisplayStatus = DisplayStatusIdle
	case StateFinished:
		view.DisplayStatus = DisplayStatusFinished
	case StatePaused:
		view.DisplayStatus = DisplayStatusPaused
	case StateRunning, StateWaiting:
		view.DisplayStatus = DisplayStatusRunning
	default:
		view.DisplayStatus = DisplayStatusIdle
	}
	return view
}
