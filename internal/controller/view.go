// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import "fmt"

type ViewStatus struct {
	DisplayStatus DisplayStatus
	Busy          bool
	NeedsInput    bool
}

func validateControllerKind(kind ControllerKind) error {
	switch kind {
	case ControllerKindWorkflow, ControllerKindService:
		return nil
	default:
		return fmt.Errorf("invalid controller kind %q", kind)
	}
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

func DeriveView(_ *Definition, state *State) ViewStatus {
	view := ViewStatus{
		DisplayStatus: DisplayStatusIdle,
		Busy:          isBusyState(state),
		NeedsInput:    state != nil && state.PendingPrompt != nil,
	}
	if state == nil {
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
