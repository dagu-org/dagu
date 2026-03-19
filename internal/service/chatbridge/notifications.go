// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
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

// NotificationEvent is a status snapshot buffered for delivery.
type NotificationEvent struct {
	Status     *exec.DAGRunStatus
	ObservedAt time.Time
}

// NotificationBatch is a flushed batch of buffered notification events.
type NotificationBatch struct {
	Class       NotificationClass
	Events      []NotificationEvent
	WindowStart time.Time
	WindowEnd   time.Time
}

type notificationBucket struct {
	id          uint64
	destination string
	class       NotificationClass
	windowStart time.Time
	events      map[string]NotificationEvent
	timer       *time.Timer
}

// NotificationBatcher buffers DAG run notifications per destination and flush window.
type NotificationBatcher struct {
	mu            sync.Mutex
	urgentWindow  time.Duration
	successWindow time.Duration
	nextBucketID  uint64
	stopped       bool
	buckets       map[string]*notificationBucket
	runIndex      map[string]string
	flush         func(destination string, batch NotificationBatch)
}

// NewNotificationBatcher creates a new notification batcher.
func NewNotificationBatcher(
	urgentWindow, successWindow time.Duration,
	flush func(destination string, batch NotificationBatch),
) *NotificationBatcher {
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
		flush:         flush,
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
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}
}

// Enqueue adds a status snapshot into the appropriate destination/window bucket.
func (b *NotificationBatcher) Enqueue(destination string, status *exec.DAGRunStatus) bool {
	if destination == "" || status == nil {
		return false
	}

	class, ok := NotificationClassForStatus(status.Status)
	if !ok {
		return false
	}

	now := time.Now()
	snapshot := cloneNotificationStatus(status)
	runKey := NotificationRunKey(snapshot)
	destRunKey := notificationDestinationRunKey(destination, runKey)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return false
	}

	if existingBucketKey, ok := b.runIndex[destRunKey]; ok {
		if existingBucket := b.buckets[existingBucketKey]; existingBucket != nil {
			if existingEvent, exists := existingBucket.events[runKey]; exists {
				if existingEvent.Status.Status == snapshot.Status {
					return true
				}
				delete(existingBucket.events, runKey)
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
			windowStart: now,
			events:      make(map[string]NotificationEvent),
		}
		b.buckets[bucketKey] = bucket
		window := b.windowForClass(class)
		bucketID := bucket.id
		bucket.timer = time.AfterFunc(window, func() {
			b.flushBucket(bucketKey, bucketID)
		})
	}

	bucket.events[runKey] = NotificationEvent{
		Status:     snapshot,
		ObservedAt: now,
	}
	b.runIndex[destRunKey] = bucketKey
	return true
}

func (b *NotificationBatcher) flushBucket(bucketKey string, bucketID uint64) {
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
	events := make([]NotificationEvent, 0, len(bucket.events))
	for runKey, event := range bucket.events {
		delete(b.runIndex, notificationDestinationRunKey(bucket.destination, runKey))
		events = append(events, event)
	}
	b.mu.Unlock()

	sort.Slice(events, func(i, j int) bool {
		if !events[i].ObservedAt.Equal(events[j].ObservedAt) {
			return events[i].ObservedAt.After(events[j].ObservedAt)
		}
		if events[i].Status.Name != events[j].Status.Name {
			return events[i].Status.Name < events[j].Status.Name
		}
		return events[i].Status.DAGRunID < events[j].Status.DAGRunID
	})

	if b.flush != nil && len(events) > 0 {
		b.flush(bucket.destination, NotificationBatch{
			Class:       bucket.class,
			Events:      events,
			WindowStart: bucket.windowStart,
			WindowEnd:   time.Now(),
		})
	}
}

func (b *NotificationBatcher) windowForClass(class NotificationClass) time.Duration {
	if class == NotificationClassUrgent {
		return b.urgentWindow
	}
	return b.successWindow
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

// NotificationBatchDAGName returns the DAG name when all events share the same DAG.
func NotificationBatchDAGName(batch NotificationBatch) string {
	if len(batch.Events) == 0 || batch.Events[0].Status == nil {
		return ""
	}
	dagName := batch.Events[0].Status.Name
	for _, event := range batch.Events[1:] {
		if event.Status == nil || event.Status.Name != dagName {
			return ""
		}
	}
	return dagName
}

// BuildNotificationPrompt constructs the single-event LLM prompt for urgent notifications.
func BuildNotificationPrompt(status *exec.DAGRunStatus) string {
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

// GenerateNotificationMessage returns the assistant message content for a flushed batch.
func GenerateNotificationMessage(
	ctx context.Context,
	svc AgentService,
	sessionID string,
	user agent.UserIdentity,
	batch NotificationBatch,
) (agent.Message, error) {
	if svc != nil && batch.Class == NotificationClassUrgent && len(batch.Events) == 1 {
		status := batch.Events[0].Status
		msg, err := svc.GenerateAssistantMessage(ctx, sessionID, user, status.Name, BuildNotificationPrompt(status))
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
		return "DAG update."
	}
	if batch.Class == NotificationClassUrgent && len(batch.Events) == 1 {
		return formatSingleNotification(batch.Events[0].Status)
	}

	groups := groupNotificationEvents(batch.Events)
	window := batch.WindowEnd.Sub(batch.WindowStart).Round(time.Second)
	if window <= 0 {
		window = time.Second
	}

	var b strings.Builder
	switch batch.Class {
	case NotificationClassUrgent:
		fmt.Fprintf(&b, "Urgent DAG updates (%d %s in last %s)\n", len(batch.Events), pluralize("run", len(batch.Events)), window)
		writeNotificationGroups(&b, groups, true)
	case NotificationClassSuccessDigest:
		fmt.Fprintf(&b, "DAG completion digest (%d %s in last %s)\n", len(batch.Events), pluralize("run", len(batch.Events)), window)
		writeNotificationGroups(&b, groups, false)
	case NotificationClassUnknown:
		fmt.Fprintf(&b, "DAG updates (%d %s)\n", len(batch.Events), pluralize("run", len(batch.Events)))
		writeNotificationGroups(&b, groups, false)
	}

	return strings.TrimSpace(b.String())
}

type notificationGroup struct {
	DAGName          string
	Status           core.Status
	Count            int
	LatestObservedAt time.Time
	Sample           *exec.DAGRunStatus
}

func groupNotificationEvents(events []NotificationEvent) []notificationGroup {
	type groupKey struct {
		dagName string
		status  core.Status
	}

	groups := make(map[groupKey]*notificationGroup)
	for _, event := range events {
		if event.Status == nil {
			continue
		}
		key := groupKey{dagName: event.Status.Name, status: event.Status.Status}
		group, ok := groups[key]
		if !ok {
			group = &notificationGroup{
				DAGName:          event.Status.Name,
				Status:           event.Status.Status,
				LatestObservedAt: event.ObservedAt,
				Sample:           event.Status,
			}
			groups[key] = group
		}
		group.Count++
		if event.ObservedAt.After(group.LatestObservedAt) {
			group.LatestObservedAt = event.ObservedAt
			group.Sample = event.Status
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
		if result[i].DAGName != result[j].DAGName {
			return result[i].DAGName < result[j].DAGName
		}
		return result[i].Status.String() < result[j].Status.String()
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
		fmt.Fprintf(b, "- %s: %s x%d", group.DAGName, group.Status.String(), group.Count)
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
		fmt.Fprintf(b, "and %d more DAG groups", hiddenCount)
	}
}

func formatSingleNotification(status *exec.DAGRunStatus) string {
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
	switch group.Status { //nolint:exhaustive // fixed notification policy
	case core.Waiting:
		return waitingNotificationDetail(group.Sample)
	case core.Failed:
		return failureNotificationDetail(group.Sample)
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
	clone := *status
	return &clone
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
