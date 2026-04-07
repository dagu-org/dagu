// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

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
	case NotificationClassInformational:
		if allDAGRun {
			fmt.Fprintf(&b, "DAG activity updates (%d %s in last %s)\n", len(batch.Events), pluralize("run", len(batch.Events)), window)
		} else {
			fmt.Fprintf(&b, "Activity updates (%d %s in last %s)\n", len(batch.Events), pluralize("event", len(batch.Events)), window)
		}
		writeNotificationGroups(&b, groups, false)
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
		return formatSingleDAGNotification(notificationDAGRun(event))
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
