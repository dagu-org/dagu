// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/dagu-org/dagu/internal/service/eventstore"
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
	NotificationClassSuccessDigest
	NotificationClassUrgent
)

// NotificationEvent is a typed subject snapshot buffered for delivery.
type NotificationEvent struct {
	Key        string                                   `json:"key"`
	Kind       eventstore.EventKind                     `json:"kind"`
	Type       eventstore.EventType                     `json:"type,omitempty"`
	Status     *exec.DAGRunStatus                       `json:"status,omitempty"`
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

type notificationBucket struct {
	id          uint64
	destination string
	class       NotificationClass
	windowStart time.Time
	events      map[string]NotificationEvent
	timer       *time.Timer
}

// NotificationBatcher buffers notification subjects per destination and flush window.
type NotificationBatcher struct {
	mu            sync.Mutex
	urgentWindow  time.Duration
	successWindow time.Duration
	nextBucketID  uint64
	stopped       bool
	buckets       map[string]*notificationBucket
	runIndex      map[string]string
	ready         []NotificationPendingBatch
	readyCh       chan struct{}
}

// NewNotificationBatcher creates a new notification batcher.
func NewNotificationBatcher(urgentWindow, successWindow time.Duration) *NotificationBatcher {
	if urgentWindow <= 0 {
		urgentWindow = DefaultUrgentNotificationWindow
	}
	if successWindow <= 0 {
		successWindow = DefaultSuccessNotificationWindow
	}
	return &NotificationBatcher{
		urgentWindow:  urgentWindow,
		successWindow: successWindow,
		buckets:       make(map[string]*notificationBucket),
		runIndex:      make(map[string]string),
		readyCh:       make(chan struct{}, 1),
	}
}

// Stop prevents future flushes and stops all pending timers.
func (b *NotificationBatcher) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	timers := make([]*time.Timer, 0, len(b.buckets))
	for _, bucket := range b.buckets {
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
	}
	b.buckets = make(map[string]*notificationBucket)
	b.runIndex = make(map[string]string)
	b.ready = nil
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}
}

// DrainAndStop prevents future flushes, stops all pending timers, and returns
// the currently buffered batches for synchronous shutdown delivery.
func (b *NotificationBatcher) DrainAndStop() []NotificationPendingBatch {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return nil
	}

	b.stopped = true
	now := time.Now()
	timers := make([]*time.Timer, 0, len(b.buckets))
	drained := make([]NotificationPendingBatch, 0, len(b.ready)+len(b.buckets))
	drained = append(drained, append([]NotificationPendingBatch(nil), b.ready...)...)
	for _, bucket := range b.buckets {
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
		batch := notificationBatchFromBucket(bucket, now)
		if len(batch.Events) == 0 {
			continue
		}
		drained = append(drained, NotificationPendingBatch{
			Destination: bucket.destination,
			Batch:       batch,
		})
	}
	b.buckets = make(map[string]*notificationBucket)
	b.runIndex = make(map[string]string)
	b.ready = nil
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}

	sort.Slice(drained, func(i, j int) bool {
		if drained[i].Batch.Class != drained[j].Batch.Class {
			return drained[i].Batch.Class == NotificationClassUrgent
		}
		if !drained[i].Batch.WindowStart.Equal(drained[j].Batch.WindowStart) {
			return drained[i].Batch.WindowStart.Before(drained[j].Batch.WindowStart)
		}
		return drained[i].Destination < drained[j].Destination
	})

	return drained
}

// Enqueue adds a notification subject into the appropriate destination/window bucket.
func (b *NotificationBatcher) Enqueue(destination string, event NotificationEvent) bool {
	if destination == "" || event.Key == "" {
		return false
	}

	class, ok := NotificationClassForEvent(event)
	if !ok {
		return false
	}

	observedAt := event.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	snapshot := cloneNotificationEvent(event)
	snapshot.ObservedAt = observedAt
	groupKey := NotificationGroupKey(snapshot)
	if groupKey == "" {
		return false
	}
	destRunKey := notificationDestinationRunKey(destination, groupKey)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return false
	}

	if existingBucketKey, ok := b.runIndex[destRunKey]; ok {
		if existingBucket := b.buckets[existingBucketKey]; existingBucket != nil {
			if existingEvent, exists := existingBucket.events[groupKey]; exists {
				if existingEvent.Key == snapshot.Key {
					return true
				}
				delete(existingBucket.events, groupKey)
				delete(b.runIndex, destRunKey)
				if len(existingBucket.events) == 0 {
					if existingBucket.timer != nil {
						existingBucket.timer.Stop()
					}
					delete(b.buckets, existingBucketKey)
				}
			}
		}
	}

	bucketKey := notificationBucketKey(destination, class)
	bucket, ok := b.buckets[bucketKey]
	if !ok {
		b.nextBucketID++
		bucket = &notificationBucket{
			id:          b.nextBucketID,
			destination: destination,
			class:       class,
			windowStart: observedAt,
			events:      make(map[string]NotificationEvent),
		}
		b.buckets[bucketKey] = bucket
		window := b.windowForClass(class)
		bucketID := bucket.id
		bucket.timer = time.AfterFunc(window, func() {
			b.readyBucket(bucketKey, bucketID)
		})
	}

	bucket.events[groupKey] = snapshot
	b.runIndex[destRunKey] = bucketKey
	return true
}

// ReadyC is signaled when one or more batches are ready for delivery.
func (b *NotificationBatcher) ReadyC() <-chan struct{} {
	return b.readyCh
}

// TakeReady returns all batches currently ready for delivery.
func (b *NotificationBatcher) TakeReady() []NotificationPendingBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.ready) == 0 {
		return nil
	}
	ready := append([]NotificationPendingBatch(nil), b.ready...)
	b.ready = nil
	return ready
}

// DiscardDestinations removes buffered and ready batches for destinations that
// are no longer configured.
func (b *NotificationBatcher) DiscardDestinations(destinations []string) {
	if len(destinations) == 0 {
		return
	}

	blocked := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		if destination == "" {
			continue
		}
		blocked[destination] = struct{}{}
	}
	if len(blocked) == 0 {
		return
	}

	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	timers := make([]*time.Timer, 0)
	for bucketKey, bucket := range b.buckets {
		if bucket == nil {
			continue
		}
		if _, ok := blocked[bucket.destination]; !ok {
			continue
		}
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
		delete(b.buckets, bucketKey)
		for runKey := range bucket.events {
			delete(b.runIndex, notificationDestinationRunKey(bucket.destination, runKey))
		}
	}

	if len(b.ready) > 0 {
		filtered := make([]NotificationPendingBatch, 0, len(b.ready))
		for _, pending := range b.ready {
			if _, ok := blocked[pending.Destination]; ok {
				continue
			}
			filtered = append(filtered, pending)
		}
		b.ready = filtered
	}
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}
}

func (b *NotificationBatcher) readyBucket(bucketKey string, bucketID uint64) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	bucket := b.buckets[bucketKey]
	if bucket == nil || bucket.id != bucketID {
		b.mu.Unlock()
		return
	}

	delete(b.buckets, bucketKey)
	for runKey := range bucket.events {
		delete(b.runIndex, notificationDestinationRunKey(bucket.destination, runKey))
	}
	batch := notificationBatchFromBucket(bucket, time.Now())
	if len(batch.Events) > 0 {
		b.ready = append(b.ready, NotificationPendingBatch{
			Destination: bucket.destination,
			Batch:       batch,
		})
		select {
		case b.readyCh <- struct{}{}:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *NotificationBatcher) windowForClass(class NotificationClass) time.Duration {
	if class == NotificationClassUrgent {
		return b.urgentWindow
	}
	return b.successWindow
}

func notificationEventKind(event NotificationEvent) eventstore.EventKind {
	if event.Kind != "" {
		return event.Kind
	}
	if event.DAGRun != nil || event.Status != nil {
		return eventstore.KindDAGRun
	}
	if event.Automata != nil {
		return eventstore.KindAutomata
	}
	return ""
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
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
		if dagRun == nil {
			return NotificationClassUnknown, false
		}
		return NotificationClassForStatus(dagRun.Status)
	case eventstore.KindAutomata:
		switch event.Type {
		case eventstore.TypeAutomataNeedsInput, eventstore.TypeAutomataError, eventstore.TypeAutomataFinished:
			return NotificationClassUrgent, true
		case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
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
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
		return NotificationSeenKey(dagRun)
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
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
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
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
		return buildDAGNotificationPrompt(dagRun)
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
		notification.Status = notification.DAGRun
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
	if snapshot.CurrentTaskDescription != "" {
		fmt.Fprintf(&prompt, "\nCurrent Task: %s", snapshot.CurrentTaskDescription)
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

// FormatNotificationBatch renders a deterministic notification message for a flushed batch.
func FormatNotificationBatch(batch NotificationBatch) string {
	if len(batch.Events) == 0 {
		return "Update."
	}
	if batch.Class == NotificationClassUrgent && len(batch.Events) == 1 {
		return formatSingleNotification(batch.Events[0])
	}

	groups := groupNotificationEvents(batch.Events)
	window := batch.WindowEnd.Sub(batch.WindowStart).Round(time.Second)
	if window <= 0 {
		window = time.Second
	}
	allDAGRun := true
	for _, event := range batch.Events {
		if notificationEventKind(event) != eventstore.KindDAGRun {
			allDAGRun = false
			break
		}
	}

	var b strings.Builder
	switch batch.Class {
	case NotificationClassUrgent:
		if allDAGRun {
			fmt.Fprintf(&b, "Urgent DAG updates (%d %s in last %s)\n", len(batch.Events), pluralize("run", len(batch.Events)), window)
		} else {
			fmt.Fprintf(&b, "Urgent updates (%d %s in last %s)\n", len(batch.Events), pluralize("event", len(batch.Events)), window)
		}
		writeNotificationGroups(&b, groups, true)
	case NotificationClassSuccessDigest:
		if allDAGRun {
			fmt.Fprintf(&b, "DAG completion digest (%d %s in last %s)\n", len(batch.Events), pluralize("run", len(batch.Events)), window)
		} else {
			fmt.Fprintf(&b, "Completion digest (%d %s in last %s)\n", len(batch.Events), pluralize("event", len(batch.Events)), window)
		}
		writeNotificationGroups(&b, groups, false)
	case NotificationClassUnknown:
		if allDAGRun {
			fmt.Fprintf(&b, "DAG updates (%d %s)\n", len(batch.Events), pluralize("run", len(batch.Events)))
		} else {
			fmt.Fprintf(&b, "Updates (%d %s)\n", len(batch.Events), pluralize("event", len(batch.Events)))
		}
		writeNotificationGroups(&b, groups, false)
	}

	return strings.TrimSpace(b.String())
}

type notificationGroup struct {
	Kind             eventstore.EventKind
	Type             eventstore.EventType
	SubjectName      string
	Status           string
	Count            int
	LatestObservedAt time.Time
	Sample           NotificationEvent
}

func groupNotificationEvents(events []NotificationEvent) []notificationGroup {
	type groupKey struct {
		kind   eventstore.EventKind
		name   string
		status string
		typ    eventstore.EventType
	}

	groups := make(map[groupKey]*notificationGroup)
	for _, event := range events {
		name := notificationSubjectName(event)
		if name == "" {
			continue
		}
		key := groupKey{
			kind:   notificationEventKind(event),
			name:   name,
			status: notificationStatusLabel(event),
			typ:    event.Type,
		}
		group, ok := groups[key]
		if !ok {
			group = &notificationGroup{
				Kind:             notificationEventKind(event),
				Type:             event.Type,
				SubjectName:      name,
				Status:           notificationStatusLabel(event),
				LatestObservedAt: event.ObservedAt,
				Sample:           cloneNotificationEvent(event),
			}
			groups[key] = group
		}
		group.Count++
		if event.ObservedAt.After(group.LatestObservedAt) {
			group.LatestObservedAt = event.ObservedAt
			group.Sample = cloneNotificationEvent(event)
		}
	}

	result := make([]notificationGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].LatestObservedAt.Equal(result[j].LatestObservedAt) {
			return result[i].LatestObservedAt.After(result[j].LatestObservedAt)
		}
		if result[i].SubjectName != result[j].SubjectName {
			return result[i].SubjectName < result[j].SubjectName
		}
		if result[i].Status != result[j].Status {
			return result[i].Status < result[j].Status
		}
		return string(result[i].Type) < string(result[j].Type)
	})
	return result
}

func writeNotificationGroups(b *strings.Builder, groups []notificationGroup, withDetails bool) {
	visible := groups
	hiddenCount := 0
	if len(groups) > maxNotificationGroups {
		visible = groups[:maxNotificationGroups]
		hiddenCount = len(groups) - maxNotificationGroups
	}

	for idx, group := range visible {
		if idx > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(b, "- %s: %s x%d", group.SubjectName, group.Status, group.Count)
		if withDetails {
			if detail := notificationGroupDetail(group); detail != "" {
				fmt.Fprintf(b, ". %s", detail)
			}
		}
	}

	if hiddenCount > 0 {
		if len(visible) > 0 {
			b.WriteByte('\n')
		}
		allDAGRun := true
		for _, group := range groups {
			if group.Kind != eventstore.KindDAGRun {
				allDAGRun = false
				break
			}
		}
		if allDAGRun {
			fmt.Fprintf(b, "and %d more DAG groups", hiddenCount)
		} else {
			fmt.Fprintf(b, "and %d more groups", hiddenCount)
		}
	}
}

func formatSingleNotification(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
		return formatSingleDAGNotification(dagRun)
	case eventstore.KindAutomata:
		return formatSingleAutomataNotification(event)
	case eventstore.KindLLMUsage:
		return "Update."
	default:
		return "Update."
	}
}

func formatSingleDAGNotification(status *exec.DAGRunStatus) string {
	if status == nil {
		return "DAG update."
	}
	var b strings.Builder
	emoji := notificationEmoji(status.Status)

	switch status.Status { //nolint:exhaustive // fixed notification policy
	case core.Waiting:
		fmt.Fprintf(&b, "%s DAG `%s` is waiting for approval.", emoji, status.Name)
		if detail := waitingNotificationDetail(status); detail != "" {
			fmt.Fprintf(&b, "\n%s", detail)
		}
	case core.Failed:
		fmt.Fprintf(&b, "%s DAG `%s` failed.", emoji, status.Name)
		if detail := failureNotificationDetail(status); detail != "" {
			fmt.Fprintf(&b, "\n%s", detail)
		}
	case core.Succeeded:
		fmt.Fprintf(&b, "%s DAG `%s` completed successfully.", emoji, status.Name)
	case core.PartiallySucceeded:
		fmt.Fprintf(&b, "%s DAG `%s` completed with partial success.", emoji, status.Name)
	default:
		fmt.Fprintf(&b, "%s DAG `%s` status: %s.", emoji, status.Name, status.Status.String())
	}

	return b.String()
}

func notificationGroupDetail(group notificationGroup) string {
	switch group.Kind {
	case eventstore.KindDAGRun:
		switch group.Sample.DAGRun.Status { //nolint:exhaustive // fixed notification policy
		case core.Waiting:
			return waitingNotificationDetail(group.Sample.DAGRun)
		case core.Failed:
			return failureNotificationDetail(group.Sample.DAGRun)
		default:
			return ""
		}
	case eventstore.KindAutomata:
		return automataNotificationDetail(group.Sample)
	case eventstore.KindLLMUsage:
		return ""
	default:
		return ""
	}
}

func notificationSubjectName(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
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
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
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
		case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
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

func formatSingleAutomataNotification(event NotificationEvent) string {
	if event.Automata == nil {
		return "Automata update."
	}
	var b strings.Builder
	snapshot := event.Automata

	switch event.Type {
	case eventstore.TypeAutomataNeedsInput:
		fmt.Fprintf(&b, "%s Automata `%s` needs input.", notificationTextEmoji(event), snapshot.Name)
		if detail := automataNotificationDetail(event); detail != "" {
			fmt.Fprintf(&b, "\n%s", detail)
		}
	case eventstore.TypeAutomataError:
		fmt.Fprintf(&b, "%s Automata `%s` hit an error.", notificationTextEmoji(event), snapshot.Name)
		if detail := automataNotificationDetail(event); detail != "" {
			fmt.Fprintf(&b, "\n%s", detail)
		}
	case eventstore.TypeAutomataFinished:
		fmt.Fprintf(&b, "%s Automata `%s` finished.", notificationTextEmoji(event), snapshot.Name)
		if detail := automataNotificationDetail(event); detail != "" {
			fmt.Fprintf(&b, "\n%s", detail)
		}
	case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
		fmt.Fprintf(&b, "%s Automata `%s` event: %s.", notificationTextEmoji(event), snapshot.Name, event.Type)
	default:
		fmt.Fprintf(&b, "%s Automata `%s` event: %s.", notificationTextEmoji(event), snapshot.Name, event.Type)
	}

	return b.String()
}

func automataNotificationDetail(event NotificationEvent) string {
	if event.Automata == nil {
		return ""
	}
	snapshot := event.Automata
	switch event.Type {
	case eventstore.TypeAutomataNeedsInput:
		if question := trimNotificationDetail(snapshot.PromptQuestion); question != "" {
			return "Prompt: " + question
		}
		return "Action is required to continue the automata."
	case eventstore.TypeAutomataError:
		if detail := trimNotificationDetail(snapshot.Error); detail != "" {
			return "Latest error: " + detail
		}
		return ""
	case eventstore.TypeAutomataFinished:
		if detail := trimNotificationDetail(snapshot.Summary); detail != "" {
			return detail
		}
		return ""
	case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
		return ""
	default:
		return ""
	}
}

func failureNotificationDetail(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	if detail := strings.TrimSpace(status.Error); detail != "" {
		return "Latest error: " + trimNotificationDetail(detail)
	}
	for _, node := range status.Nodes {
		if node == nil || strings.TrimSpace(node.Error) == "" {
			continue
		}
		return fmt.Sprintf("Latest error at %s: %s", node.Step.Name, trimNotificationDetail(node.Error))
	}
	for _, handler := range []*exec.Node{status.OnFailure, status.OnExit} {
		if handler == nil || strings.TrimSpace(handler.Error) == "" {
			continue
		}
		stepName := handler.Step.Name
		if stepName == "" {
			stepName = "handler"
		}
		return fmt.Sprintf("Latest error at %s: %s", stepName, trimNotificationDetail(handler.Error))
	}
	return ""
}

func waitingNotificationDetail(status *exec.DAGRunStatus) string {
	if status == nil {
		return ""
	}
	for _, node := range status.Nodes {
		if node == nil || node.Status != core.NodeWaiting {
			continue
		}
		if node.Step.Name != "" {
			return fmt.Sprintf("Waiting at step %s.", node.Step.Name)
		}
	}
	if status.OnWait != nil && status.OnWait.Step.Name != "" {
		return fmt.Sprintf("Waiting at step %s.", status.OnWait.Step.Name)
	}
	if detail := strings.TrimSpace(status.Error); detail != "" {
		return trimNotificationDetail(detail)
	}
	return "Action is required to resume the DAG."
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
	dagRun := event.DAGRun
	if dagRun == nil {
		dagRun = event.Status
	}
	cloned := NotificationEvent{
		Key:        event.Key,
		Kind:       event.Kind,
		Type:       event.Type,
		ObservedAt: event.ObservedAt,
		DAGRun:     cloneNotificationStatus(dagRun),
	}
	cloned.Status = cloned.DAGRun
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

func trimNotificationDetail(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxNotificationDetailRunes {
		return text
	}
	return string(runes[:maxNotificationDetailRunes-1]) + "…"
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func notificationEmoji(status core.Status) string {
	switch status { //nolint:exhaustive // only notified statuses are handled
	case core.Succeeded, core.PartiallySucceeded:
		return "\u2705"
	case core.Failed:
		return "\u274C"
	case core.Waiting:
		return "\u23F3"
	default:
		return "\u2139\uFE0F"
	}
}

func notificationTextEmoji(event NotificationEvent) string {
	switch notificationEventKind(event) {
	case eventstore.KindDAGRun:
		dagRun := event.DAGRun
		if dagRun == nil {
			dagRun = event.Status
		}
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
		case eventstore.TypeDAGRunWaiting, eventstore.TypeDAGRunSucceeded, eventstore.TypeDAGRunFailed, eventstore.TypeDAGRunAborted, eventstore.TypeDAGRunRejected, eventstore.TypeLLMUsageRecorded:
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
