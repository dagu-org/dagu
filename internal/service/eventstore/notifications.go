// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
)

const (
	dagRunStatusSnapshotDataKey         = "dag_run_status"
	notificationStatusSnapshotDataKey   = "notification_status"
	notificationAutomataSnapshotDataKey = "notification_automata"
	DAGFileNameDataKey                  = "dag_file"
)

type DAGRunCursor struct {
	LastInboxFile    string           `json:"last_inbox_file,omitempty"`
	CommittedOffsets map[string]int64 `json:"committed_offsets,omitempty"`
}

func (c DAGRunCursor) Normalize() DAGRunCursor {
	if c.CommittedOffsets == nil {
		c.CommittedOffsets = make(map[string]int64)
	}
	return c
}

func (c DAGRunCursor) Equal(other DAGRunCursor) bool {
	return c.LastInboxFile == other.LastInboxFile && maps.Equal(c.CommittedOffsets, other.CommittedOffsets)
}

type DAGRunReader interface {
	DAGRunHeadCursor(ctx context.Context) (DAGRunCursor, error)
	ReadDAGRunEvents(ctx context.Context, cursor DAGRunCursor) ([]*Event, DAGRunCursor, error)
}

type NotificationCursor = DAGRunCursor

type NotificationReader interface {
	NotificationHeadCursor(ctx context.Context) (NotificationCursor, error)
	ReadNotificationEvents(ctx context.Context, cursor NotificationCursor) ([]*Event, NotificationCursor, error)
}

type DAGRunNodeSnapshot struct {
	StepName string          `json:"step_name,omitempty"`
	Status   core.NodeStatus `json:"status,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type NotificationNodeSnapshot = DAGRunNodeSnapshot

func newDAGRunNodeSnapshot(node *exec.Node) *DAGRunNodeSnapshot {
	if node == nil {
		return nil
	}
	return &DAGRunNodeSnapshot{
		StepName: node.Step.Name,
		Status:   node.Status,
		Error:    node.Error,
	}
}

func (s *DAGRunNodeSnapshot) Node() *exec.Node {
	if s == nil {
		return nil
	}
	return &exec.Node{
		Step:   core.Step{Name: s.StepName},
		Status: s.Status,
		Error:  s.Error,
	}
}

type DAGRunRefSnapshot struct {
	Name     string `json:"name,omitempty"`
	DAGRunID string `json:"dag_run_id,omitempty"`
}

func newDAGRunRefSnapshot(ref exec.DAGRunRef) DAGRunRefSnapshot {
	return DAGRunRefSnapshot{
		Name:     ref.Name,
		DAGRunID: ref.ID,
	}
}

func (s DAGRunRefSnapshot) DAGRunRef() exec.DAGRunRef {
	if s.Name == "" || s.DAGRunID == "" {
		return exec.DAGRunRef{}
	}
	return exec.NewDAGRunRef(s.Name, s.DAGRunID)
}

type DAGRunStatusSnapshot struct {
	Root       DAGRunRefSnapshot    `json:"root"`
	Parent     DAGRunRefSnapshot    `json:"parent"`
	Name       string               `json:"name"`
	DAGFile    string               `json:"dag_file,omitempty"`
	DAGRunID   string               `json:"dag_run_id"`
	AttemptID  string               `json:"attempt_id"`
	ProcGroup  string               `json:"proc_group,omitempty"`
	Status     core.Status          `json:"status"`
	Error      string               `json:"error,omitempty"`
	Log        string               `json:"log,omitempty"`
	QueuedAt   string               `json:"queued_at,omitempty"`
	StartedAt  string               `json:"started_at,omitempty"`
	FinishedAt string               `json:"finished_at,omitempty"`
	Nodes      []DAGRunNodeSnapshot `json:"nodes,omitempty"`
	OnFailure  *DAGRunNodeSnapshot  `json:"on_failure,omitempty"`
	OnExit     *DAGRunNodeSnapshot  `json:"on_exit,omitempty"`
	OnWait     *DAGRunNodeSnapshot  `json:"on_wait,omitempty"`
}

type NotificationStatusSnapshot = DAGRunStatusSnapshot

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

func (s *DAGRunStatusSnapshot) Validate() error {
	if s == nil {
		return errors.New("eventstore: dag-run snapshot is nil")
	}
	if s.DAGRunID == "" {
		return errors.New("eventstore: invalid dag-run snapshot: missing dag_run_id")
	}
	if s.AttemptID == "" {
		return errors.New("eventstore: invalid dag-run snapshot: missing attempt_id")
	}
	if s.Name == "" {
		return errors.New("eventstore: invalid dag-run snapshot: missing name")
	}
	switch s.Status { //nolint:exhaustive // persisted DAG-run events only allow lifecycle states
	case core.Queued, core.Running, core.Waiting, core.Succeeded, core.PartiallySucceeded, core.Failed, core.Aborted, core.Rejected:
	default:
		return errors.New("eventstore: invalid dag-run snapshot: missing or unsupported status")
	}
	return nil
}

func newDAGRunStatusSnapshot(status *exec.DAGRunStatus, dagFile string) *DAGRunStatusSnapshot {
	if status == nil {
		return nil
	}

	nodes := make([]DAGRunNodeSnapshot, 0, len(status.Nodes))
	for _, node := range status.Nodes {
		snapshot := newDAGRunNodeSnapshot(node)
		if snapshot == nil {
			continue
		}
		nodes = append(nodes, *snapshot)
	}

	return &DAGRunStatusSnapshot{
		Root:       newDAGRunRefSnapshot(status.Root),
		Parent:     newDAGRunRefSnapshot(status.Parent),
		Name:       status.Name,
		DAGFile:    dagFile,
		DAGRunID:   status.DAGRunID,
		AttemptID:  status.AttemptID,
		ProcGroup:  status.ProcGroup,
		Status:     status.Status,
		Error:      status.Error,
		Log:        status.Log,
		QueuedAt:   status.QueuedAt,
		StartedAt:  status.StartedAt,
		FinishedAt: status.FinishedAt,
		Nodes:      nodes,
		OnFailure:  newDAGRunNodeSnapshot(status.OnFailure),
		OnExit:     newDAGRunNodeSnapshot(status.OnExit),
		OnWait:     newDAGRunNodeSnapshot(status.OnWait),
	}
}

func newNotificationAutomataSnapshot(eventType EventType, input AutomataEventInput) *NotificationAutomataSnapshot {
	if input.Name == "" {
		return nil
	}
	return &NotificationAutomataSnapshot{
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
}

func (s *DAGRunStatusSnapshot) DAGRunStatus() *exec.DAGRunStatus {
	if s == nil {
		return nil
	}

	nodes := make([]*exec.Node, 0, len(s.Nodes))
	for _, node := range s.Nodes {
		nodes = append(nodes, node.Node())
	}

	return &exec.DAGRunStatus{
		Root:       s.Root.DAGRunRef(),
		Parent:     s.Parent.DAGRunRef(),
		Name:       s.Name,
		DAGRunID:   s.DAGRunID,
		AttemptID:  s.AttemptID,
		Status:     s.Status,
		ProcGroup:  s.ProcGroup,
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

func IsDAGRunEventType(kind EventKind, eventType EventType) bool {
	if kind != KindDAGRun {
		return false
	}
	switch eventType {
	case TypeDAGRunQueued, TypeDAGRunRunning, TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected:
		return true
	case TypeLLMUsageRecorded:
		return false
	default:
		return false
	}
}

func IsNotificationEventType(kind EventKind, eventType EventType) bool {
	switch kind {
	case KindDAGRun:
		if !IsDAGRunEventType(kind, eventType) {
			return false
		}
		switch eventType {
		case TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected:
			return true
		case TypeDAGRunQueued, TypeDAGRunRunning, TypeLLMUsageRecorded:
			return false
		default:
			return false
		}
	case KindAutomata:
		switch eventType {
		case TypeAutomataNeedsInput, TypeAutomataError, TypeAutomataFinished:
			return true
		case TypeDAGRunQueued, TypeDAGRunRunning, TypeDAGRunWaiting, TypeDAGRunSucceeded, TypeDAGRunFailed, TypeDAGRunAborted, TypeDAGRunRejected, TypeLLMUsageRecorded:
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

func DAGRunStatusFromEvent(event *Event) (*exec.DAGRunStatus, error) {
	snapshot, err := DAGRunSnapshotFromEvent(event)
	if err != nil {
		return nil, err
	}
	return snapshot.DAGRunStatus(), nil
}

func DAGRunSnapshotFromEvent(event *Event) (*DAGRunStatusSnapshot, error) {
	if event == nil {
		return nil, errors.New("eventstore: event is nil")
	}
	if !IsDAGRunEventType(event.Kind, event.Type) {
		return nil, fmt.Errorf("eventstore: event %q is not a dag-run event", event.Type)
	}
	return dagRunSnapshotFromData(event.Data)
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
		snapshot, err := dagRunSnapshotFromData(event.Data)
		if err != nil {
			return nil, err
		}
		payload.DAGRun = (*NotificationStatusSnapshot)(snapshot)
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

func dagRunSnapshotFromData(data map[string]any) (*DAGRunStatusSnapshot, error) {
	snapshot, err := dagRunStatusSnapshotFromData(data)
	if err != nil {
		return nil, err
	}
	if snapshot.DAGFile == "" {
		snapshot.DAGFile = dagRunFileNameFromData(data)
	}
	return snapshot, nil
}

func dagRunStatusSnapshotFromData(data map[string]any) (*DAGRunStatusSnapshot, error) {
	if len(data) == 0 {
		return nil, errors.New("eventstore: dag-run snapshot is missing")
	}

	raw, ok := data[dagRunStatusSnapshotDataKey]
	if !ok {
		raw, ok = data[notificationStatusSnapshotDataKey]
	}
	if !ok {
		return nil, errors.New("eventstore: dag-run snapshot is missing")
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("eventstore: marshal dag-run snapshot: %w", err)
	}

	var snapshot DAGRunStatusSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, fmt.Errorf("eventstore: unmarshal dag-run snapshot: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func dagRunFileNameFromData(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	raw, ok := data[DAGFileNameDataKey]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

func (s *Service) DAGRunHeadCursor(ctx context.Context) (DAGRunCursor, error) {
	if s == nil || s.store == nil {
		return DAGRunCursor{}, errors.New("eventstore: store is not configured")
	}
	if reader, ok := s.store.(DAGRunReader); ok {
		cursor, err := reader.DAGRunHeadCursor(ctx)
		if err != nil {
			return DAGRunCursor{}, err
		}
		return cursor.Normalize(), nil
	}
	if reader, ok := s.store.(NotificationReader); ok {
		cursor, err := reader.NotificationHeadCursor(ctx)
		if err != nil {
			return DAGRunCursor{}, err
		}
		return DAGRunCursor(cursor).Normalize(), nil
	}
	return DAGRunCursor{}, errors.New("eventstore: dag-run reader is not configured")
}

func (s *Service) ReadDAGRunEvents(ctx context.Context, cursor DAGRunCursor) ([]*Event, DAGRunCursor, error) {
	if s == nil || s.store == nil {
		return nil, DAGRunCursor{}, errors.New("eventstore: store is not configured")
	}
	cursor = cursor.Normalize()
	if reader, ok := s.store.(DAGRunReader); ok {
		events, nextCursor, err := reader.ReadDAGRunEvents(ctx, cursor)
		if err != nil {
			return nil, DAGRunCursor{}, err
		}
		return events, nextCursor.Normalize(), nil
	}
	if reader, ok := s.store.(NotificationReader); ok {
		events, nextCursor, err := reader.ReadNotificationEvents(ctx, NotificationCursor(cursor))
		if err != nil {
			return nil, DAGRunCursor{}, err
		}
		return events, DAGRunCursor(nextCursor).Normalize(), nil
	}
	return nil, DAGRunCursor{}, errors.New("eventstore: dag-run reader is not configured")
}

func (s *Service) NotificationHeadCursor(ctx context.Context) (NotificationCursor, error) {
	cursor, err := s.DAGRunHeadCursor(ctx)
	return NotificationCursor(cursor), err
}

func (s *Service) ReadNotificationEvents(ctx context.Context, cursor NotificationCursor) ([]*Event, NotificationCursor, error) {
	events, nextCursor, err := s.ReadDAGRunEvents(ctx, DAGRunCursor(cursor))
	if err != nil {
		return nil, NotificationCursor{}, err
	}
	filtered := make([]*Event, 0, len(events))
	for _, event := range events {
		if event == nil || !IsNotificationEventType(event.Kind, event.Type) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered, NotificationCursor(nextCursor), nil
}
