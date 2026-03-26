// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type LifecycleState string

const (
	StateIdle     LifecycleState = "idle"
	StateRunning  LifecycleState = "running"
	StateWaiting  LifecycleState = "waiting"
	StateFinished LifecycleState = "finished"
)

type WaitingReason string

const (
	WaitingReasonNone  WaitingReason = ""
	WaitingReasonHuman WaitingReason = "human_input"
	WaitingReasonDAG   WaitingReason = "dag_wait"
)

type AllowedDAGs struct {
	Names []string `json:"names,omitempty" yaml:"names,omitempty"`
	Tags  []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type AgentConfig struct {
	Model         string   `json:"model,omitempty" yaml:"model,omitempty"`
	Soul          string   `json:"soul,omitempty" yaml:"soul,omitempty"`
	EnabledSkills []string `json:"enabledSkills,omitempty" yaml:"enabledSkills,omitempty"`
	SafeMode      bool     `json:"safeMode,omitempty" yaml:"safeMode,omitempty"`
}

type ScheduleList []core.Schedule

func (s *ScheduleList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		*s = nil
		return nil
	}

	var expressions []string
	switch value.Kind {
	case yaml.ScalarNode:
		expressions = []string{strings.TrimSpace(value.Value)}
	case yaml.SequenceNode:
		expressions = make([]string, 0, len(value.Content))
		for _, node := range value.Content {
			if node.Kind != yaml.ScalarNode {
				return fmt.Errorf("schedule entries must be strings")
			}
			expressions = append(expressions, strings.TrimSpace(node.Value))
		}
	default:
		return fmt.Errorf("schedule must be a string or list of strings")
	}

	if len(expressions) == 0 {
		*s = nil
		return nil
	}

	out := make([]core.Schedule, 0, len(expressions))
	for _, expr := range expressions {
		if expr == "" {
			continue
		}
		parsed, err := cron.ParseStandard(expr)
		if err != nil {
			return fmt.Errorf("invalid schedule %q: %w", expr, err)
		}
		out = append(out, core.Schedule{
			Expression: expr,
			Parsed:     parsed,
		})
	}
	*s = out
	return nil
}

type Definition struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Purpose     string       `json:"purpose" yaml:"purpose"`
	Goal        string       `json:"goal" yaml:"goal"`
	Stages      []string     `json:"stages" yaml:"stages"`
	Schedule    ScheduleList `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	AllowedDAGs AllowedDAGs  `json:"allowedDAGs" yaml:"allowedDAGs"`
	Agent       AgentConfig  `json:"agent,omitempty" yaml:"agent,omitempty"`
	Disabled    bool         `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

type Prompt struct {
	ID                  string                   `json:"id"`
	Question            string                   `json:"question"`
	Options             []agent.UserPromptOption `json:"options,omitempty"`
	AllowFreeText       bool                     `json:"allowFreeText,omitempty"`
	FreeTextPlaceholder string                   `json:"freeTextPlaceholder,omitempty"`
	CreatedAt           time.Time                `json:"createdAt"`
}

type PromptResponse struct {
	PromptID          string    `json:"promptId"`
	SelectedOptionIDs []string  `json:"selectedOptionIds,omitempty"`
	FreeTextResponse  string    `json:"freeTextResponse,omitempty"`
	RespondedAt       time.Time `json:"respondedAt"`
}

type State struct {
	State              LifecycleState  `json:"state"`
	CurrentStage       string          `json:"currentStage,omitempty"`
	StageChangedAt     time.Time       `json:"stageChangedAt,omitempty"`
	StageChangedBy     string          `json:"stageChangedBy,omitempty"`
	StageNote          string          `json:"stageNote,omitempty"`
	SessionID          string          `json:"sessionId,omitempty"`
	CurrentRunRef      *exec.DAGRunRef `json:"currentRunRef,omitempty"`
	LastRunRef         *exec.DAGRunRef `json:"lastRunRef,omitempty"`
	CurrentCycleID     string          `json:"currentCycleId,omitempty"`
	WaitingReason      WaitingReason   `json:"waitingReason,omitempty"`
	PendingPrompt      *Prompt         `json:"pendingPrompt,omitempty"`
	PendingResponse    *PromptResponse `json:"pendingResponse,omitempty"`
	StartRequestedAt   time.Time       `json:"startRequestedAt,omitempty"`
	LastTriggeredAt    time.Time       `json:"lastTriggeredAt,omitempty"`
	LastScheduleMinute time.Time       `json:"lastScheduleMinute,omitempty"`
	LastUpdatedAt      time.Time       `json:"lastUpdatedAt,omitempty"`
	FinishedAt         time.Time       `json:"finishedAt,omitempty"`
	LastSummary        string          `json:"lastSummary,omitempty"`
	LastError          string          `json:"lastError,omitempty"`
}

type AllowedDAGInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type RunSummary struct {
	Name        string    `json:"name"`
	DAGRunID    string    `json:"dagRunId"`
	Status      string    `json:"status"`
	TriggerType string    `json:"triggerType,omitempty"`
	StartedAt   string    `json:"startedAt,omitempty"`
	FinishedAt  string    `json:"finishedAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type Summary struct {
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Purpose       string         `json:"purpose"`
	Goal          string         `json:"goal"`
	State         LifecycleState `json:"state"`
	Stage         string         `json:"stage,omitempty"`
	Disabled      bool           `json:"disabled,omitempty"`
	CurrentRun    *RunSummary    `json:"currentRun,omitempty"`
	LastUpdatedAt time.Time      `json:"lastUpdatedAt,omitempty"`
}

type Detail struct {
	Definition  *Definition      `json:"definition"`
	State       *State           `json:"state"`
	AllowedDAGs []AllowedDAGInfo `json:"allowedDags"`
	CurrentRun  *RunSummary      `json:"currentRun,omitempty"`
	RecentRuns  []RunSummary     `json:"recentRuns,omitempty"`
	Messages    []agent.Message  `json:"messages,omitempty"`
}

type StartRequest struct {
	RequestedBy string `json:"requestedBy,omitempty"`
}

type StageOverrideRequest struct {
	Stage       string `json:"stage"`
	RequestedBy string `json:"requestedBy,omitempty"`
	Note        string `json:"note,omitempty"`
}

type HumanResponseRequest struct {
	PromptID          string   `json:"promptId"`
	SelectedOptionIDs []string `json:"selectedOptionIds,omitempty"`
	FreeTextResponse  string   `json:"freeTextResponse,omitempty"`
}

func newInitialState(def *Definition) *State {
	stage := ""
	if def != nil && len(def.Stages) > 0 {
		stage = def.Stages[0]
	}
	now := time.Now()
	return &State{
		State:          StateIdle,
		CurrentStage:   stage,
		StageChangedAt: now,
		StageChangedBy: "system",
		LastUpdatedAt:  now,
	}
}

func nextCycleID() string {
	return uuid.NewString()
}
