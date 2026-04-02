// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const (
	notificationStatusSnapshotDataKey   = "notification_status"
	notificationAutomataSnapshotDataKey = "notification_automata"
)

type NotificationCursor struct {
	LastInboxFile    string           `json:"last_inbox_file,omitempty"`
	CommittedOffsets map[string]int64 `json:"committed_offsets,omitempty"`
}

func (c NotificationCursor) Normalize() NotificationCursor {
	if c.CommittedOffsets == nil {
		c.CommittedOffsets = make(map[string]int64)
	}
	return c
}

func (c NotificationCursor) Equal(other NotificationCursor) bool {
	return c.LastInboxFile == other.LastInboxFile && maps.Equal(c.CommittedOffsets, other.CommittedOffsets)
}

type NotificationReader interface {
	NotificationHeadCursor(ctx context.Context) (NotificationCursor, error)
	ReadNotificationEvents(ctx context.Context, cursor NotificationCursor) ([]*Event, NotificationCursor, error)
}

type NotificationNodeSnapshot struct {
	StepName string          `json:"step_name,omitempty"`
	Status   core.NodeStatus `json:"status,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func newNotificationNodeSnapshot(node *exec.Node) *NotificationNodeSnapshot {
	if node == nil {
		return nil
	}
	return &NotificationNodeSnapshot{
		StepName: node.Step.Name,
		Status:   node.Status,
		Error:    node.Error,
	}
}

func (s *NotificationNodeSnapshot) Node() *exec.Node {
	if s == nil {
		return nil
	}
	return &exec.Node{
		Step:   core.Step{Name: s.StepName},
		Status: s.Status,
		Error:  s.Error,
	}
}

type NotificationStatusSnapshot struct {
	Name       string                     `json:"name"`
	DAGRunID   string                     `json:"dag_run_id"`
	AttemptID  string                     `json:"attempt_id"`
	Status     core.Status                `json:"status"`
	Error      string                     `json:"error,omitempty"`
	Log        string                     `json:"log,omitempty"`
	QueuedAt   string                     `json:"queued_at,omitempty"`
	StartedAt  string                     `json:"started_at,omitempty"`
	FinishedAt string                     `json:"finished_at,omitempty"`
	Nodes      []NotificationNodeSnapshot `json:"nodes,omitempty"`
	OnFailure  *NotificationNodeSnapshot  `json:"on_failure,omitempty"`
	OnExit     *NotificationNodeSnapshot  `json:"on_exit,omitempty"`
	OnWait     *NotificationNodeSnapshot  `json:"on_wait,omitempty"`
}

type NotificationAutomataSnapshot struct {
	Name                   string    `json:"name"`
	Kind                   string    `json:"kind,omitempty"`
	CycleID                string    `json:"cycle_id,omitempty"`
	EventType              EventType `json:"event_type"`
	Status                 string    `json:"status,omitempty"`
	PromptID               string    `json:"prompt_id,omitempty"`
	PromptQuestion         string    `json:"prompt_question,omitempty"`
	Summary                string    `json:"summary,omitempty"`
	Error                  string    `json:"error,omitempty"`
	CurrentTaskDescription string    `json:"current_task_description,omitempty"`
	OpenTaskCount          int       `json:"open_task_count,omitempty"`
	DoneTaskCount          int       `json:"done_task_count,omitempty"`
}

func (s *NotificationAutomataSnapshot) Validate() error {
	if s == nil {
		return errors.New("eventstore: automata notification snapshot is nil")
	}
	if s.Name == "" {
		return errors.New("eventstore: invalid automata notification snapshot: missing name")
	}
	switch s.EventType {
	case TypeAutomataNeedsInput, TypeAutomataError, TypeAutomataFinished:
	case TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected, TypeLLMUsageRecorded:
		return errors.New("eventstore: invalid automata notification snapshot: unsupported event type")
	default:
		return errors.New("eventstore: invalid automata notification snapshot: unsupported event type")
	}
	return nil
}

func (s *NotificationStatusSnapshot) Validate() error {
	if s == nil {
		return errors.New("eventstore: notification snapshot is nil")
	}
	if s.DAGRunID == "" {
		return errors.New("eventstore: invalid notification snapshot: missing dag_run_id")
	}
	if s.AttemptID == "" {
		return errors.New("eventstore: invalid notification snapshot: missing attempt_id")
	}
	if s.Name == "" {
		return errors.New("eventstore: invalid notification snapshot: missing name")
	}
	switch s.Status { //nolint:exhaustive // notification snapshots only allow persisted terminal/waiting states
	case core.Waiting, core.Succeeded, core.PartiallySucceeded, core.Failed, core.Aborted, core.Rejected:
	default:
		return errors.New("eventstore: invalid notification snapshot: missing or unsupported status")
	}
	return nil
}

func newNotificationStatusSnapshot(status *exec.DAGRunStatus) *NotificationStatusSnapshot {
	if status == nil {
		return nil
	}

	nodes := make([]NotificationNodeSnapshot, 0, len(status.Nodes))
	for _, node := range status.Nodes {
		snapshot := newNotificationNodeSnapshot(node)
		if snapshot == nil {
			continue
		}
		nodes = append(nodes, *snapshot)
	}

	return &NotificationStatusSnapshot{
		Name:       status.Name,
		DAGRunID:   status.DAGRunID,
		AttemptID:  status.AttemptID,
		Status:     status.Status,
		Error:      status.Error,
		Log:        status.Log,
		QueuedAt:   status.QueuedAt,
		StartedAt:  status.StartedAt,
		FinishedAt: status.FinishedAt,
		Nodes:      nodes,
		OnFailure:  newNotificationNodeSnapshot(status.OnFailure),
		OnExit:     newNotificationNodeSnapshot(status.OnExit),
		OnWait:     newNotificationNodeSnapshot(status.OnWait),
	}
}

func newNotificationAutomataSnapshot(eventType EventType, input AutomataEventInput) *NotificationAutomataSnapshot {
	if input.Name == "" {
		return nil
	}
	snapshot := &NotificationAutomataSnapshot{
		Name:                   input.Name,
		Kind:                   input.Kind,
		CycleID:                input.CycleID,
		EventType:              eventType,
		Status:                 input.Status,
		PromptID:               input.PromptID,
		PromptQuestion:         input.PromptQuestion,
		Summary:                input.Summary,
		Error:                  input.Error,
		CurrentTaskDescription: input.CurrentTaskDescription,
		OpenTaskCount:          input.OpenTaskCount,
		DoneTaskCount:          input.DoneTaskCount,
	}
	return snapshot
}

func (s *NotificationStatusSnapshot) DAGRunStatus() *exec.DAGRunStatus {
	if s == nil {
		return nil
	}

	nodes := make([]*exec.Node, 0, len(s.Nodes))
	for _, node := range s.Nodes {
		nodes = append(nodes, node.Node())
	}

	return &exec.DAGRunStatus{
		Name:       s.Name,
		DAGRunID:   s.DAGRunID,
		AttemptID:  s.AttemptID,
		Status:     s.Status,
		Error:      s.Error,
		Log:        s.Log,
		QueuedAt:   s.QueuedAt,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Nodes:      nodes,
		OnFailure:  s.OnFailure.Node(),
		OnExit:     s.OnExit.Node(),
		OnWait:     s.OnWait.Node(),
	}
}

type NotificationPayload struct {
	Kind     EventKind
	Type     EventType
	DAGRun   *NotificationStatusSnapshot
	Automata *NotificationAutomataSnapshot
}

func IsNotificationEventType(kind EventKind, eventType EventType) bool {
	switch kind {
	case KindDAGRun:
		switch eventType {
		case TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected:
			return true
		case TypeAutomataNeedsInput, TypeAutomataError, TypeAutomataFinished, TypeLLMUsageRecorded:
			return false
		default:
			return false
		}
	case KindAutomata:
		switch eventType {
		case TypeAutomataNeedsInput, TypeAutomataError, TypeAutomataFinished:
			return true
		case TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected, TypeLLMUsageRecorded:
			return false
		default:
			return false
		}
	case KindLLMUsage:
		return false
	default:
		return false
	}
}

func NotificationPayloadFromEvent(event *Event) (*NotificationPayload, error) {
	if event == nil {
		return nil, errors.New("eventstore: event is nil")
	}
	if !IsNotificationEventType(event.Kind, event.Type) {
		return nil, fmt.Errorf("eventstore: event %q is not a notification event", event.Type)
	}
	if len(event.Data) == 0 {
		return nil, errors.New("eventstore: notification snapshot is missing")
	}

	payload := &NotificationPayload{
		Kind: event.Kind,
		Type: event.Type,
	}
	switch event.Kind {
	case KindDAGRun:
		raw, ok := event.Data[notificationStatusSnapshotDataKey]
		if !ok {
			return nil, errors.New("eventstore: notification snapshot is missing")
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("eventstore: marshal notification snapshot: %w", err)
		}
		var snapshot NotificationStatusSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("eventstore: unmarshal notification snapshot: %w", err)
		}
		if err := snapshot.Validate(); err != nil {
			return nil, err
		}
		payload.DAGRun = &snapshot
		return payload, nil
	case KindAutomata:
		raw, ok := event.Data[notificationAutomataSnapshotDataKey]
		if !ok {
			return nil, errors.New("eventstore: automata notification snapshot is missing")
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("eventstore: marshal automata notification snapshot: %w", err)
		}
		var snapshot NotificationAutomataSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("eventstore: unmarshal automata notification snapshot: %w", err)
		}
		if err := snapshot.Validate(); err != nil {
			return nil, err
		}
		payload.Automata = &snapshot
		return payload, nil
	case KindLLMUsage:
		return nil, fmt.Errorf("eventstore: unsupported notification kind %q", event.Kind)
	default:
		return nil, fmt.Errorf("eventstore: unsupported notification kind %q", event.Kind)
	}
}

func NotificationStatusFromEvent(event *Event) (*exec.DAGRunStatus, error) {
	payload, err := NotificationPayloadFromEvent(event)
	if err != nil {
		return nil, err
	}
	if payload.DAGRun == nil {
		return nil, errors.New("eventstore: notification payload does not contain a dag snapshot")
	}
	return payload.DAGRun.DAGRunStatus(), nil
}

func NotificationAutomataFromEvent(event *Event) (*NotificationAutomataSnapshot, error) {
	payload, err := NotificationPayloadFromEvent(event)
	if err != nil {
		return nil, err
	}
	if payload.Automata == nil {
		return nil, errors.New("eventstore: notification payload does not contain an automata snapshot")
	}
	return payload.Automata, nil
}

func (s *Service) NotificationHeadCursor(ctx context.Context) (NotificationCursor, error) {
	if s == nil || s.store == nil {
		return NotificationCursor{}, errors.New("eventstore: store is not configured")
	}
	reader, ok := s.store.(NotificationReader)
	if !ok {
		return NotificationCursor{}, errors.New("eventstore: notification reader is not configured")
	}
	cursor, err := reader.NotificationHeadCursor(ctx)
	if err != nil {
		return NotificationCursor{}, err
	}
	return cursor.Normalize(), nil
}

func (s *Service) ReadNotificationEvents(ctx context.Context, cursor NotificationCursor) ([]*Event, NotificationCursor, error) {
	if s == nil || s.store == nil {
		return nil, NotificationCursor{}, errors.New("eventstore: store is not configured")
	}
	reader, ok := s.store.(NotificationReader)
	if !ok {
		return nil, NotificationCursor{}, errors.New("eventstore: notification reader is not configured")
	}
	cursor = cursor.Normalize()
	events, nextCursor, err := reader.ReadNotificationEvents(ctx, cursor)
	if err != nil {
		return nil, NotificationCursor{}, err
	}
	return events, nextCursor.Normalize(), nil
}
