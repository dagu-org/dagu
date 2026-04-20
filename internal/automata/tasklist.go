// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"fmt"
	"strings"
	"time"
)

func normalizePersistedTaskTemplates(tasks *[]TaskTemplate) (bool, error) {
	if tasks == nil {
		return false, nil
	}
	if *tasks == nil {
		*tasks = []TaskTemplate{}
		return true, nil
	}

	changed := false
	seen := make(map[string]struct{}, len(*tasks))
	for i := range *tasks {
		task := &(*tasks)[i]
		task.Description = strings.TrimSpace(task.Description)
		if task.Description == "" {
			return false, fmt.Errorf("task template %d has an empty description", i+1)
		}
		if task.ID == "" {
			return false, fmt.Errorf("task template %d is missing an id", i+1)
		}
		if _, ok := seen[task.ID]; ok {
			return false, fmt.Errorf("duplicate task template id %q", task.ID)
		}
		seen[task.ID] = struct{}{}
		if task.CreatedAt.IsZero() {
			task.CreatedAt = task.UpdatedAt
			changed = true
		}
		if task.UpdatedAt.IsZero() {
			task.UpdatedAt = task.CreatedAt
			changed = true
		}
	}
	return changed, nil
}

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

func cloneTaskTemplatesFromTasks(tasks []Task) []TaskTemplate {
	if len(tasks) == 0 {
		return []TaskTemplate{}
	}
	out := make([]TaskTemplate, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, TaskTemplate{
			ID:          task.ID,
			Description: task.Description,
			CreatedAt:   firstNonZeroTime(task.CreatedAt, task.UpdatedAt),
			CreatedBy:   task.CreatedBy,
			UpdatedAt:   firstNonZeroTime(task.UpdatedAt, task.CreatedAt),
			UpdatedBy:   task.UpdatedBy,
		})
	}
	return out
}

func cloneTasksFromTemplates(tasks []TaskTemplate, now time.Time) []Task {
	if len(tasks) == 0 {
		return []Task{}
	}
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, Task{
			ID:          task.ID,
			Description: task.Description,
			State:       TaskStateOpen,
			CreatedAt:   now,
			CreatedBy:   task.CreatedBy,
			UpdatedAt:   now,
			UpdatedBy:   task.UpdatedBy,
		})
	}
	return out
}

func taskFromTemplate(task TaskTemplate) Task {
	return Task{
		ID:          task.ID,
		Description: task.Description,
		State:       TaskStateOpen,
		CreatedAt:   task.CreatedAt,
		CreatedBy:   task.CreatedBy,
		UpdatedAt:   task.UpdatedAt,
		UpdatedBy:   task.UpdatedBy,
	}
}

func findTaskTemplateIndex(tasks []TaskTemplate, taskID string) int {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return i
		}
	}
	return -1
}

func hasTaskTemplates(tasks []TaskTemplate) bool {
	return len(tasks) > 0
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
