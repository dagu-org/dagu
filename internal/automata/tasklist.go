// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"fmt"
	"strings"
	"time"
)

func normalizePersistedTasks(tasks *[]Task) (bool, error) {
	if tasks == nil {
		return false, nil
	}
	if *tasks == nil {
		*tasks = []Task{}
		return true, nil
	}

	changed := false
	seen := make(map[string]struct{}, len(*tasks))
	for i := range *tasks {
		task := &(*tasks)[i]
		task.Description = strings.TrimSpace(task.Description)
		if task.Description == "" {
			return false, fmt.Errorf("task %d has an empty description", i+1)
		}
		if task.ID == "" {
			return false, fmt.Errorf("task %d is missing an id", i+1)
		}
		if _, ok := seen[task.ID]; ok {
			return false, fmt.Errorf("duplicate task id %q", task.ID)
		}
		seen[task.ID] = struct{}{}
		switch task.State {
		case "":
			task.State = TaskStateOpen
			changed = true
		case TaskStateOpen:
			task.DoneAt = time.Time{}
			task.DoneBy = ""
		case TaskStateDone:
		default:
			return false, fmt.Errorf("task %q has invalid state %q", task.ID, task.State)
		}
	}
	return changed, nil
}

func countTasksByState(tasks []Task, state TaskState) int {
	count := 0
	for _, task := range tasks {
		if task.State == state {
			count++
		}
	}
	return count
}

func findTaskIndex(tasks []Task, taskID string) int {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return i
		}
	}
	return -1
}

func hasOpenTask(tasks []Task) bool {
	for _, task := range tasks {
		if task.State == TaskStateOpen {
			return true
		}
	}
	return false
}

func buildTaskListSummary(tasks []Task) string {
	if len(tasks) == 0 {
		return "- no tasks"
	}
	var sb strings.Builder
	for _, task := range tasks {
		marker := "[ ]"
		if task.State == TaskStateDone {
			marker = "[x]"
		}
		fmt.Fprintf(&sb, "- %s %s\n", marker, task.Description)
	}
	return strings.TrimRight(sb.String(), "\n")
}
