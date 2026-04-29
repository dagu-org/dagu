// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/auth"
	"github.com/dagucloud/dagu/internal/autopilot"
	"github.com/dagucloud/dagu/internal/service/audit"
)

const (
	autopilotMemoryReflectionMaxCurrentMemory = 12000
	autopilotMemoryReflectionMaxMessages      = 80
	autopilotMemoryReflectionMaxMessageChars  = 4000
)

// ReflectAutopilotMemory generates a proposed MEMORY.md update from the current
// Autopilot session transcript. The proposal is preview-only; callers save it
// with UpdateAutopilotDocument after review.
func (a *API) ReflectAutopilotMemory(ctx context.Context, request api.ReflectAutopilotMemoryRequestObject) (api.ReflectAutopilotMemoryResponseObject, error) {
	if err := a.requireAutopilotService(); err != nil {
		return nil, err
	}
	if err := a.requireAutopilotDocumentStore(); err != nil {
		return nil, err
	}
	if err := a.requireAgent(ctx); err != nil {
		return nil, err
	}

	name := string(request.Name)
	detail, err := a.autopilotService.Detail(ctx, name)
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}
	if err := a.requireDAGWriteForWorkspace(ctx, autopilotWorkspaceNameFromDetail(detail)); err != nil {
		return nil, err
	}
	if detail.State == nil || strings.TrimSpace(detail.State.SessionID) == "" || len(detail.Messages) == 0 {
		return nil, autopilotMemoryReflectionBadRequest("Autopilot has no session transcript to reflect on")
	}

	current, err := a.autopilotService.GetDocument(ctx, name, autopilot.DocumentMemory)
	if err != nil {
		return nil, toAutopilotAPIError(err)
	}

	prompt := buildAutopilotMemoryReflectionPrompt(detail, current.Content)
	msg, err := a.agentAPI.GenerateAssistantMessage(ctx, detail.State.SessionID, autopilotMemoryReflectionUser(name), "", prompt)
	if err != nil {
		return nil, mapAgentError(err)
	}
	proposed, rationale, err := parseAutopilotMemoryReflection(msg.Content)
	if err != nil {
		return nil, autopilotMemoryReflectionInternalError(err.Error())
	}

	a.logAudit(ctx, audit.CategoryAutopilot, "memory_reflect", map[string]any{"name": name})
	return api.ReflectAutopilotMemory200JSONResponse{
		CurrentContent:  current.Content,
		Name:            name,
		ProposedContent: proposed,
		Rationale:       rationale,
	}, nil
}

type autopilotMemoryReflectionPayload struct {
	ProposedContent string `json:"proposedContent"`
	Rationale       string `json:"rationale"`
}

func autopilotMemoryReflectionUser(name string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   "__autopilot__:" + name,
		Username: "autopilot/" + name,
		Role:     auth.RoleAdmin,
	}
}

func buildAutopilotMemoryReflectionPrompt(detail *autopilot.Detail, currentContent string) string {
	var b strings.Builder
	b.WriteString("You are updating an Autopilot-specific MEMORY.md document from one Autopilot session.\n")
	b.WriteString("Return strict JSON only with this shape: {\"proposedContent\":\"...\",\"rationale\":\"...\"}.\n")
	b.WriteString("The proposedContent value must be the full replacement content for MEMORY.md.\n")
	b.WriteString("Keep durable lessons, operating preferences, stable constraints, and reusable procedures.\n")
	b.WriteString("Retain useful existing memory unless the session clearly supersedes it.\n")
	b.WriteString("Exclude temporary task status, transient errors, raw secrets, and run-specific noise unless they reveal a reusable rule.\n")
	b.WriteString("Keep the memory concise and directly useful for future Autopilot turns.\n\n")

	if detail.Definition != nil {
		fmt.Fprintf(&b, "Autopilot: %s\n", detail.Definition.Name)
		if detail.Definition.Goal != "" {
			fmt.Fprintf(&b, "Goal: %s\n", detail.Definition.Goal)
		}
		if detail.Definition.StandingInstruction != "" {
			fmt.Fprintf(&b, "Standing instruction: %s\n", detail.Definition.StandingInstruction)
		}
	}
	if detail.State != nil {
		if detail.State.Instruction != "" {
			fmt.Fprintf(&b, "Current operator instruction: %s\n", detail.State.Instruction)
		}
		if detail.State.LastSummary != "" {
			fmt.Fprintf(&b, "Last summary: %s\n", detail.State.LastSummary)
		}
		if detail.State.LastError != "" {
			fmt.Fprintf(&b, "Last error: %s\n", detail.State.LastError)
		}
		if len(detail.State.Tasks) > 0 {
			b.WriteString("\nTasks:\n")
			for i, task := range detail.State.Tasks {
				state := string(task.State)
				if state == "" {
					state = "open"
				}
				fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, state, truncateForAutopilotReflectionPrompt(task.Description, autopilotMemoryReflectionMaxMessageChars))
			}
		}
	}

	current := strings.TrimSpace(currentContent)
	if current == "" {
		current = "(empty)"
	}
	fmt.Fprintf(&b, "\n<current_memory>\n%s\n</current_memory>\n", truncateForAutopilotReflectionPrompt(current, autopilotMemoryReflectionMaxCurrentMemory))
	fmt.Fprintf(&b, "\n<session_transcript>\n%s\n</session_transcript>\n", buildAutopilotMemoryReflectionTranscript(detail.Messages))
	return b.String()
}

func buildAutopilotMemoryReflectionTranscript(messages []agent.Message) string {
	if len(messages) > autopilotMemoryReflectionMaxMessages {
		messages = messages[len(messages)-autopilotMemoryReflectionMaxMessages:]
	}
	var b strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(autopilotMemoryReflectionMessageContent(message))
		if content == "" {
			continue
		}
		fmt.Fprintf(&b, "[%d] %s", message.SequenceID, message.Type)
		if !message.CreatedAt.IsZero() {
			fmt.Fprintf(&b, " %s", message.CreatedAt.Format(time.RFC3339))
		}
		b.WriteString("\n")
		b.WriteString(truncateForAutopilotReflectionPrompt(content, autopilotMemoryReflectionMaxMessageChars))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(b.String()) == "" {
		return "(empty)"
	}
	return strings.TrimSpace(b.String())
}

func autopilotMemoryReflectionMessageContent(message agent.Message) string {
	var parts []string
	if strings.TrimSpace(message.Content) != "" {
		parts = append(parts, message.Content)
	}
	if message.UserPrompt != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "Prompt: %s", message.UserPrompt.Question)
		if len(message.UserPrompt.Options) > 0 {
			b.WriteString("\nOptions:")
			for _, option := range message.UserPrompt.Options {
				fmt.Fprintf(&b, "\n- %s: %s", option.ID, option.Label)
				if option.Description != "" {
					fmt.Fprintf(&b, " (%s)", option.Description)
				}
			}
		}
		parts = append(parts, b.String())
	}
	if len(message.ToolCalls) > 0 {
		names := make([]string, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			names = append(names, call.Function.Name)
		}
		parts = append(parts, "Tool calls: "+strings.Join(names, ", "))
	}
	if len(message.ToolResults) > 0 {
		var b strings.Builder
		for _, result := range message.ToolResults {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("Tool result")
			if result.ToolCallID != "" {
				fmt.Fprintf(&b, " %s", result.ToolCallID)
			}
			if result.IsError {
				b.WriteString(" (error)")
			}
			fmt.Fprintf(&b, ": %s", result.Content)
		}
		parts = append(parts, b.String())
	}
	if message.UIAction != nil {
		parts = append(parts, fmt.Sprintf("UI action: %s %s", message.UIAction.Type, message.UIAction.Path))
	}
	return strings.Join(parts, "\n")
}

func parseAutopilotMemoryReflection(content string) (string, string, error) {
	raw := strings.TrimSpace(content)
	if raw == "" {
		return "", "", fmt.Errorf("assistant returned empty reflection")
	}
	raw = trimAutopilotReflectionJSONFence(raw)
	payload, err := decodeAutopilotMemoryReflection(raw)
	if err != nil {
		return "", "", err
	}
	proposed := strings.TrimSpace(payload.ProposedContent)
	if proposed == "" {
		return "", "", fmt.Errorf("assistant returned empty proposed memory")
	}
	return proposed, strings.TrimSpace(payload.Rationale), nil
}

func decodeAutopilotMemoryReflection(raw string) (autopilotMemoryReflectionPayload, error) {
	var payload autopilotMemoryReflectionPayload
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		return payload, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &payload); err == nil {
			return payload, nil
		}
	}
	return autopilotMemoryReflectionPayload{}, fmt.Errorf("assistant returned invalid reflection JSON")
}

func trimAutopilotReflectionJSONFence(raw string) string {
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		return raw
	}
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func truncateForAutopilotReflectionPrompt(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "\n[truncated]"
}

func autopilotMemoryReflectionBadRequest(message string) error {
	return &Error{
		Code:       api.ErrorCodeBadRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

func autopilotMemoryReflectionInternalError(message string) error {
	return &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}
