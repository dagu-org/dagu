// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/dagucloud/dagu/internal/auth"
)

//go:embed system_prompt.txt
var systemPromptRaw string

// systemPromptTemplate is parsed once at package initialization.
var systemPromptTemplate = template.Must(
	template.New("system_prompt").Parse(systemPromptRaw),
)

// CurrentDAG contains context about the DAG being viewed.
type CurrentDAG struct {
	Name     string
	FilePath string
	RunID    string
	Status   string
}

// UserCapabilities contains role and capability context for the current user.
type UserCapabilities struct {
	Role           string
	CanExecuteDAGs bool
	CanWriteDAGs   bool
	CanViewAudit   bool
	IsAdmin        bool
}

// soulPlaceholder is an internal marker that replaces soul content during
// template execution to prevent user-controlled content from being interpreted
// as Go template directives.
const soulPlaceholder = "\x00__SOUL_CONTENT__\x00"

// systemPromptData contains all data for template rendering.
type systemPromptData struct {
	EnvironmentInfo
	CurrentDAG  *CurrentDAG
	Memory      MemoryContent
	User        *UserCapabilities
	SoulContent string
}

// SystemPromptParams holds all parameters for system prompt generation.
type SystemPromptParams struct {
	Env        EnvironmentInfo
	CurrentDAG *CurrentDAG
	Memory     MemoryContent
	Role       auth.Role
	Soul       *Soul
}

// GenerateSystemPrompt renders the system prompt template with the given parameters.
func GenerateSystemPrompt(p SystemPromptParams) string {
	env := p.Env
	currentDAG := p.CurrentDAG
	memory := p.Memory
	role := p.Role
	soul := p.Soul
	var buf bytes.Buffer
	var rawSoulContent string
	if soul != nil {
		rawSoulContent = soul.Content
	}
	// Use a placeholder during template execution to prevent user-controlled
	// soul content from being interpreted as Go template directives.
	templateSoulContent := soulPlaceholder
	if rawSoulContent == "" {
		templateSoulContent = ""
	}
	data := systemPromptData{
		EnvironmentInfo: env,
		CurrentDAG:      currentDAG,
		Memory:          memory,
		User:            buildUserCapabilities(role),
		SoulContent:     templateSoulContent,
	}
	if err := systemPromptTemplate.Execute(&buf, data); err != nil {
		return fallbackPrompt(env)
	}
	result := buf.String()
	if rawSoulContent != "" {
		result = strings.Replace(result, soulPlaceholder, rawSoulContent, 1)
	}
	return result
}

func buildUserCapabilities(role auth.Role) *UserCapabilities {
	if role == "" {
		return nil
	}
	return &UserCapabilities{
		Role:           role.String(),
		CanExecuteDAGs: role.CanExecute(),
		CanWriteDAGs:   role.CanWrite(),
		CanViewAudit:   role.CanManageAudit(),
		IsAdmin:        role.IsAdmin(),
	}
}

// fallbackPrompt returns a basic prompt when template execution fails.
func fallbackPrompt(env EnvironmentInfo) string {
	return "You are Dagu Assistant, an AI assistant for DAG workflows. DAGs Directory: " + env.DAGsDir
}
