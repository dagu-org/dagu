// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"maps"
	"slices"
	"time"
)

func cloneNotificationMonitorState(state notificationMonitorState) notificationMonitorState {
	clone := notificationMonitorState{
		Version:      state.Version,
		Bootstrapped: state.Bootstrapped,
		SourceCursor: state.SourceCursor.Normalize(),
		Destinations: make(map[string]*notificationDestinationState, len(state.Destinations)),
	}
	clone.SourceCursor.CommittedOffsets = maps.Clone(state.SourceCursor.CommittedOffsets)
	for destination, destState := range state.Destinations {
		if destState == nil {
			clone.Destinations[destination] = &notificationDestinationState{
				Pending:   make(map[string]NotificationEvent),
				Delivered: make(map[string]time.Time),
			}
			continue
		}
		pending := make(map[string]NotificationEvent, len(destState.Pending))
		for key, event := range destState.Pending {
			pending[key] = cloneNotificationEvent(event)
		}
		clone.Destinations[destination] = &notificationDestinationState{
			Pending:   pending,
			Delivered: maps.Clone(destState.Delivered),
		}
	}
	clone.normalize()
	return clone
}

func ensureDestinations(state *notificationMonitorState, destinations []string) bool {
	changed := false
	for _, destination := range destinations {
		if destination == "" {
			continue
		}
		if _, ok := state.Destinations[destination]; ok {
			continue
		}
		state.Destinations[destination] = &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
		changed = true
	}
	return changed
}

func reconcileDestinations(state *notificationMonitorState, destinations []string) ([]string, bool) {
	allowed := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		if destination == "" {
			continue
		}
		allowed[destination] = struct{}{}
	}

	changed := ensureDestinations(state, destinations)
	removed := make([]string, 0)
	for destination := range state.Destinations {
		if _, ok := allowed[destination]; ok {
			continue
		}
		delete(state.Destinations, destination)
		removed = append(removed, destination)
		changed = true
	}
	slices.Sort(removed)
	return removed, changed
}

func destinationState(state *notificationMonitorState, destination string) *notificationDestinationState {
	if destination == "" {
		return &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
	}
	current, ok := state.Destinations[destination]
	if !ok || current == nil {
		current = &notificationDestinationState{
			Pending:   make(map[string]NotificationEvent),
			Delivered: make(map[string]time.Time),
		}
		state.Destinations[destination] = current
	}
	if current.Pending == nil {
		current.Pending = make(map[string]NotificationEvent)
	}
	if current.Delivered == nil {
		current.Delivered = make(map[string]time.Time)
	}
	return current
}

func enqueueNotifications(state *notificationMonitorState, destinations []string, events []NotificationEvent) ([]queuedNotification, bool, bool) {
	var (
		queued   []queuedNotification
		changed  bool
		accepted bool
	)

	for _, destination := range destinations {
		destState := destinationState(state, destination)
		for _, event := range events {
			if event.Key == "" {
				continue
			}
			if _, ok := destState.Delivered[event.Key]; ok {
				continue
			}
			if pending, ok := destState.Pending[event.Key]; ok {
				queued = append(queued, queuedNotification{
					destination: destination,
					event:       pending,
				})
				accepted = true
				continue
			}

			groupKey := NotificationGroupKey(event)
			if groupKey == "" {
				continue
			}
			for pendingKey, pending := range destState.Pending {
				if NotificationGroupKey(pending) != groupKey || pendingKey == event.Key {
					continue
				}
				delete(destState.Pending, pendingKey)
			}

			destState.Pending[event.Key] = cloneNotificationEvent(event)
			queued = append(queued, queuedNotification{
				destination: destination,
				event:       event,
			})
			accepted = true
			changed = true
		}
	}

	return queued, changed, accepted
}
