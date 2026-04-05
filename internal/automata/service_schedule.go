// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"strings"
	"time"
)

func (s *Service) HandleScheduleTick(ctx context.Context, tickTime time.Time) error {
	defs, err := s.ListDefinitions(ctx)
	if err != nil {
		return err
	}
	tickTime = tickTime.Truncate(time.Minute)
	for _, def := range defs {
		if err := s.handleScheduledServiceTick(ctx, def, tickTime); err != nil {
			s.logger.Warn("automata schedule tick failed",
				"automata", def.Name,
				"error", err,
			)
		}
	}
	return nil
}

func (s *Service) handleScheduledServiceTick(ctx context.Context, def *Definition, tickTime time.Time) error {
	if def == nil || def.Disabled || !isService(def) || len(def.Schedule) == 0 {
		return nil
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.State == StatePaused {
		return nil
	}
	if !state.LastScheduleMinute.IsZero() && state.LastScheduleMinute.Equal(tickTime) {
		return nil
	}
	if !scheduleListDueAt(def.Schedule, tickTime) {
		return nil
	}
	state.LastScheduleMinute = tickTime
	if strings.TrimSpace(def.StandingInstruction) == "" {
		return s.recordScheduleConfigError(ctx, def, state, tickTime, "service automata schedule requires a standing instruction")
	}
	if !hasTaskTemplates(state.TaskTemplates) {
		return s.recordScheduleConfigError(ctx, def, state, tickTime, "service automata schedule requires at least one task template")
	}
	if state.PendingPrompt != nil || state.CurrentRunRef != nil || len(state.PendingTurnMessages) > 0 {
		return s.saveState(ctx, def.Name, state)
	}
	activity := s.inspectSessionActivity(ctx, def.Name, state)
	if activity.Working || activity.HasPendingPrompt || activity.HasQueuedInput {
		return s.saveState(ctx, def.Name, state)
	}
	if err := s.startServiceCycle(ctx, def, state, strings.TrimSpace(def.StandingInstruction), "schedule"); err != nil {
		return err
	}
	queueTurnMessage(state, "scheduled_tick", s.buildScheduledTickMessage(def, state, tickTime), s.clock())
	state.LastScheduleMinute = tickTime
	return s.saveState(ctx, def.Name, state)
}

func (s *Service) recordScheduleConfigError(ctx context.Context, def *Definition, state *State, tickTime time.Time, message string) error {
	if state == nil {
		return nil
	}
	prevError := state.LastError
	state.LastError = message
	state.LastScheduleMinute = tickTime
	if err := s.saveState(ctx, def.Name, state); err != nil {
		return err
	}
	if prevError != message {
		s.eventEmitter().error(ctx, def, state, "schedule_config_invalid", tickTime)
	}
	return nil
}

func scheduleListDueAt(items ScheduleList, tickTime time.Time) bool {
	for _, item := range items {
		if _, due := item.DueAt(tickTime); due {
			return true
		}
	}
	return false
}
