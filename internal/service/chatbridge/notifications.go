// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/llm"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

const (
	DefaultUrgentNotificationWindow  = 15 * time.Second
	DefaultSuccessNotificationWindow = 2 * time.Minute
	maxNotificationGroups            = 12
	maxNotificationDetailRunes       = 160
)

// NotificationStatuses are the DAG statuses that can generate bot notifications.
var NotificationStatuses = []core.Status{
	core.Succeeded,
	core.Failed,
	core.PartiallySucceeded,
	core.Waiting,
}

// NotificationClass controls batching policy.
type NotificationClass int

const (
	NotificationClassUnknown NotificationClass = iota
	NotificationClassInformational
	NotificationClassSuccessDigest
	NotificationClassUrgent
)

// NotificationEvent is a typed subject snapshot buffered for delivery.
type NotificationEvent struct {
	Key        string                                   `json:"key"`
	Kind       eventstore.EventKind                     `json:"kind"`
	Type       eventstore.EventType                     `json:"type,omitempty"`
	DAGRun     *exec.DAGRunStatus                       `json:"dagRun,omitempty"`
	Automata   *eventstore.NotificationAutomataSnapshot `json:"automata,omitempty"`
	ObservedAt time.Time                                `json:"observedAt"`
}

// NotificationBatch is a flushed batch of buffered notification events.
type NotificationBatch struct {
	Class       NotificationClass
	Events      []NotificationEvent
	WindowStart time.Time
	WindowEnd   time.Time
}

// NotificationPendingBatch is a pending batch drained during monitor shutdown.
type NotificationPendingBatch struct {
	Destination string
	Batch       NotificationBatch
}

func notificationEventKind(event NotificationEvent) eventstore.EventKind {
	if event.Kind != "" {
		return event.Kind
	}
	if event.DAGRun != nil {
		return eventstore.KindDAGRun
	}
	if event.Automata != nil {
		return eventstore.KindAutomata
	}
	return ""
}

func notificationDAGRun(event NotificationEvent) *exec.DAGRunStatus {
	return event.DAGRun
}

// NotificationClassForStatus maps a DAG status to its notification class.
func NotificationClassForStatus(status core.Status) (NotificationClass, bool) {
	switch status { //nolint:exhaustive // notification policy is intentionally fixed
	case core.Failed, core.Waiting:
		return NotificationClassUrgent, true
	case core.Succeeded, core.PartiallySucceeded:
		return NotificationClassSuccessDigest, true
	default:
		return NotificationClassUnknown, false
	}
}

// NotificationClassForEvent maps a buffered notification event to its batching class.
func NotificationClassForEvent(event NotificationEvent) (NotificationClass, bool) {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := notificationDAGRun(event)
		if dagRun == nil {
			return NotificationClassUnknown, false
		}
		switch event.Type {
		case eventstore.TypeDAGRunQueued, eventstore.TypeDAGRunRunning:
			return NotificationClassInformational, true
		case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected:
			return NotificationClassUrgent, true
		case eventstore.TypeDAGRunSucceeded:
			return NotificationClassSuccessDigest, true
		case eventstore.TypeLLMUsageRecorded:
			return NotificationClassUnknown, false
		default:
			return NotificationClassForStatus(dagRun.Status)
		}
	case eventstore.KindAutomata:
		switch event.Type {
		case eventstore.TypeAutomataNeedsInput, eventstore.TypeAutomataError, eventstore.TypeAutomataFinished:
			return NotificationClassUrgent, true
		case eventstore.TypeDAGRunQueued, eventstore.TypeDAGRunRunning, eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
			return NotificationClassUnknown, false
		default:
			return NotificationClassUnknown, false
		}
	case eventstore.KindLLMUsage:
		return NotificationClassUnknown, false
	default:
		return NotificationClassUnknown, false
	}
}

// NotificationSeenKey is used by monitors to suppress repeated polling of the same status.
func NotificationSeenKey(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	return NotificationRunKey(status) + ":" + status.Status.String()
}

// NotificationRunKey identifies a DAG run attempt independent of the latest status.
func NotificationRunKey(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	return status.DAGRunID + ":" + status.AttemptID
}

// NotificationSeenKeyForEvent is used by monitors to suppress repeated polling of the same notification subject.
func NotificationSeenKeyForEvent(event NotificationEvent) string {
	if event.Key != "" {
		return event.Key
	}
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		return NotificationSeenKey(notificationDAGRun(event))
	case eventstore.KindAutomata:
		if event.Automata == nil {
			return ""
		}
		return "automata:" + string(event.Type) + ":" + event.Automata.Name + ":" + event.Automata.CycleID
	case eventstore.KindLLMUsage:
		return ""
	default:
		return ""
	}
}

// NotificationGroupKey identifies a buffered subject independent of the latest status update.
func NotificationGroupKey(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := notificationDAGRun(event)
		if dagRun == nil {
			return ""
		}
		return "dag:" + NotificationRunKey(dagRun)
	case eventstore.KindAutomata:
		return "automata:" + NotificationSeenKeyForEvent(event)
	case eventstore.KindLLMUsage:
		return ""
	default:
		return ""
	}
}

// NotificationBatchTopicName returns a shared subject name when all events target the same subject.
func NotificationBatchTopicName(batch NotificationBatch) string {
	if len(batch.Events) == 0 {
		return ""
	}
	name := notificationSubjectName(batch.Events[0])
	if name == "" {
		return ""
	}
	for _, event := range batch.Events[1:] {
		if notificationSubjectName(event) != name {
			return ""
		}
	}
	return name
}

// NotificationBatchDAGName preserves the old DAG-specific helper behavior.
func NotificationBatchDAGName(batch NotificationBatch) string {
	if len(batch.Events) == 0 || notificationEventKind(batch.Events[0]) != eventstore.KindDAGRun {
		return ""
	}
	return NotificationBatchTopicName(batch)
}

// BuildNotificationPrompt constructs the single-event LLM prompt for urgent notifications.
func BuildNotificationPrompt(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		return buildDAGNotificationPrompt(notificationDAGRun(event))
	case eventstore.KindAutomata:
		return buildAutomataNotificationPrompt(event)
	case eventstore.KindLLMUsage:
		return ""
	default:
		return ""
	}
}

func NotificationEventFromStoredEvent(event *eventstore.Event) (NotificationEvent, error) {
	if event == nil {
		return NotificationEvent{}, fmt.Errorf("notification event is nil")
	}
	payload, err := eventstore.NotificationPayloadFromEvent(event)
	if err != nil {
		return NotificationEvent{}, err
	}
	observedAt := event.RecordedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	notification := NotificationEvent{
		Kind:       payload.Kind,
		Type:       payload.Type,
		ObservedAt: observedAt.UTC(),
	}
	switch payload.Kind {
	case eventstore.KindDAGRun:
		if payload.DAGRun == nil {
			return NotificationEvent{}, fmt.Errorf("dag notification snapshot is missing")
		}
		notification.DAGRun = payload.DAGRun.DAGRunStatus()
		notification.Key = NotificationSeenKey(notification.DAGRun)
	case eventstore.KindAutomata:
		if payload.Automata == nil {
			return NotificationEvent{}, fmt.Errorf("automata notification snapshot is missing")
		}
		notification.Automata = payload.Automata
		notification.Key = event.ID
	case eventstore.KindLLMUsage:
		return NotificationEvent{}, fmt.Errorf("unsupported notification kind %q", payload.Kind)
	default:
		return NotificationEvent{}, fmt.Errorf("unsupported notification kind %q", payload.Kind)
	}
	return notification, nil
}

func buildDAGNotificationPrompt(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}

	var intro string
	if status.Status == core.Waiting {
		intro = "A DAG run is waiting for human approval. Please write a brief, urgent notification message for the user. Let them know which steps are waiting and that action is needed. Keep it concise (2-4 sentences)."
	} else {
		intro = "A DAG run just completed. Please write a brief, helpful notification message for the user about this event. Keep it concise (2-4 sentences). Include the key facts and any actionable information."
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, `%s

DAG Name: %s
Status: %s
DAG Run ID: %s`, intro, status.Name, status.Status.String(), status.DAGRunID)

	if status.Error != "" {
		fmt.Fprintf(&prompt, "\nError: %s", status.Error)
	}
	if status.StartedAt != "" {
		fmt.Fprintf(&prompt, "\nStarted: %s", status.StartedAt)
	}
	if status.FinishedAt != "" {
		fmt.Fprintf(&prompt, "\nFinished: %s", status.FinishedAt)
	}
	if status.Log != "" {
		fmt.Fprintf(&prompt, "\nLog file: %s", status.Log)
	}
	if len(status.Nodes) > 0 {
		prompt.WriteString("\n\nStep results:")
		for _, node := range status.Nodes {
			line := fmt.Sprintf("\n- %s: %s", node.Step.Name, node.Status.String())
			if node.Error != "" {
				line += fmt.Sprintf(" (error: %s)", node.Error)
			}
			prompt.WriteString(line)
		}
	}
	prompt.WriteString("\n\nWrite a notification message. Do NOT use tools or execute any commands. Just write the message text directly.")
	return prompt.String()
}

func buildAutomataNotificationPrompt(event NotificationEvent) string {
	if event.Automata == nil {
		return ""
	}

	snapshot := event.Automata
	intro := "An automata generated an urgent runtime event. Please write a brief, helpful notification message for the user. Keep it concise (2-4 sentences). Include the key facts and any immediate action."
	if event.Type == eventstore.TypeAutomataNeedsInput {
		intro = "An automata needs human input. Please write a brief, urgent notification message for the user. Explain what needs input and what question they need to answer. Keep it concise (2-4 sentences)."
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, `%s

Automata Name: %s
Automata Kind: %s
Event Type: %s`, intro, snapshot.Name, snapshot.Kind, event.Type)

	if snapshot.CycleID != "" {
		fmt.Fprintf(&prompt, "\nCycle ID: %s", snapshot.CycleID)
	}
	if snapshot.Status != "" {
		fmt.Fprintf(&prompt, "\nLifecycle State: %s", snapshot.Status)
	}
	if snapshot.PromptQuestion != "" {
		fmt.Fprintf(&prompt, "\nPrompt: %s", snapshot.PromptQuestion)
	}
	if snapshot.Error != "" {
		fmt.Fprintf(&prompt, "\nError: %s", snapshot.Error)
	}
	if snapshot.Summary != "" {
		fmt.Fprintf(&prompt, "\nSummary: %s", snapshot.Summary)
	}
	if snapshot.OpenTaskCount > 0 || snapshot.DoneTaskCount > 0 {
		fmt.Fprintf(&prompt, "\nTasks: %d open, %d done", snapshot.OpenTaskCount, snapshot.DoneTaskCount)
	}
	prompt.WriteString("\n\nWrite a notification message. Do NOT use tools or execute any commands. Just write the message text directly.")
	return prompt.String()
}

// GenerateNotificationMessage returns the assistant message content for a flushed batch.
func GenerateNotificationMessage(
	ctx context.Context,
	svc AgentService,
	sessionID string,
	user agent.UserIdentity,
	batch NotificationBatch,
) (agent.Message, error) {
	if svc != nil && batch.Class == NotificationClassUrgent && len(batch.Events) == 1 {
		event := batch.Events[0]
		msg, err := svc.GenerateAssistantMessage(ctx, sessionID, user, notificationSubjectName(event), BuildNotificationPrompt(event))
		if err == nil {
			return msg, nil
		}
		return newNotificationMessage(FormatNotificationBatch(batch)), err
	}

	return newNotificationMessage(FormatNotificationBatch(batch)), nil
}

func notificationSubjectName(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := notificationDAGRun(event)
		if dagRun == nil {
			return ""
		}
		return dagRun.Name
	case eventstore.KindAutomata:
		if event.Automata == nil {
			return ""
		}
		return event.Automata.Name
	case eventstore.KindLLMUsage:
		return ""
	default:
		return ""
	}
}

func NotificationSubjectName(event NotificationEvent) string {
	return notificationSubjectName(event)
}

func notificationStatusLabel(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := notificationDAGRun(event)
		if dagRun == nil {
			return string(event.Type)
		}
		return dagRun.Status.String()
	case eventstore.KindAutomata:
		switch event.Type {
		case eventstore.TypeAutomataNeedsInput:
			return "needs_input"
		case eventstore.TypeAutomataError:
			return "error"
		case eventstore.TypeAutomataFinished:
			return "finished"
		case eventstore.TypeDAGRunQueued, eventstore.TypeDAGRunRunning, eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
			return string(event.Type)
		default:
			return string(event.Type)
		}
	case eventstore.KindLLMUsage:
		return string(event.Type)
	default:
		return string(event.Type)
	}
}

func NotificationStatusLabel(event NotificationEvent) string {
	return notificationStatusLabel(event)
}

func newNotificationMessage(content string) agent.Message {
	return agent.Message{
		Type:      agent.MessageTypeAssistant,
		Content:   content,
		CreatedAt: time.Now(),
		LLMData: &llm.Message{
			Role:    llm.RoleAssistant,
			Content: content,
		},
	}
}

func cloneNotificationStatus(status *exec.DAGRunStatus) *exec.DAGRunStatus {
	if status == nil {
		return nil
	}
	// Notification batches may stay buffered for minutes, so take an isolated
	// snapshot rather than sharing mutable slices or handler pointers from the
	// store object.
	data, err := json.Marshal(status)
	if err != nil {
		clone := *status
		return &clone
	}

	var clone exec.DAGRunStatus
	if err := json.Unmarshal(data, &clone); err != nil {
		fallback := *status
		return &fallback
	}
	return &clone
}

func cloneNotificationEvent(event NotificationEvent) NotificationEvent {
	cloned := NotificationEvent{
		Key:        event.Key,
		Kind:       event.Kind,
		Type:       event.Type,
		ObservedAt: event.ObservedAt,
		DAGRun:     cloneNotificationStatus(notificationDAGRun(event)),
	}
	if event.Automata != nil {
		snapshot := *event.Automata
		cloned.Automata = &snapshot
	}
	return cloned
}

func notificationBatchFromBucket(bucket *notificationBucket, windowEnd time.Time) NotificationBatch {
	events := make([]NotificationEvent, 0, len(bucket.events))
	for _, event := range bucket.events {
		events = append(events, event)
	}
	sortNotificationEvents(events)
	return NotificationBatch{
		Class:       bucket.class,
		Events:      events,
		WindowStart: bucket.windowStart,
		WindowEnd:   windowEnd,
	}
}

func sortNotificationEvents(events []NotificationEvent) {
	sort.Slice(events, func(i, j int) bool {
		if !events[i].ObservedAt.Equal(events[j].ObservedAt) {
			return events[i].ObservedAt.After(events[j].ObservedAt)
		}
		leftName := notificationSubjectName(events[i])
		rightName := notificationSubjectName(events[j])
		if leftName != rightName {
			return leftName < rightName
		}
		return events[i].Key < events[j].Key
	})
}

func notificationBucketKey(destination string, class NotificationClass) string {
	return fmt.Sprintf("%s|%d", destination, class)
}

func notificationDestinationRunKey(destination, runKey string) string {
	return destination + "|" + runKey
}

func notificationTextEmoji(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := notificationDAGRun(event)
		if dagRun == nil {
			return "\u2139\uFE0F"
		}
		return notificationEmoji(dagRun.Status)
	case eventstore.KindAutomata:
		switch event.Type {
		case eventstore.TypeAutomataFinished:
			return "\u2705"
		case eventstore.TypeAutomataNeedsInput:
			return "\u23F3"
		case eventstore.TypeAutomataError:
			return "\u274C"
		case eventstore.TypeDAGRunQueued, eventstore.TypeDAGRunRunning, eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
			return "\u2139\uFE0F"
		default:
			return "\u2139\uFE0F"
		}
	case eventstore.KindLLMUsage:
		return "\u2139\uFE0F"
	default:
		return "\u2139\uFE0F"
	}
}
