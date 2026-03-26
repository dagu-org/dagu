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
	StatePaused   LifecycleState = "paused"
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

type StageDefinition struct {
	Name        string      `json:"name" yaml:"name"`
	AllowedDAGs AllowedDAGs `json:"allowedDAGs,omitempty" yaml:"allowed_dags,omitempty"`
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
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Purpose     string            `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Goal        string            `json:"goal" yaml:"goal"`
	Stages      []StageDefinition `json:"stages" yaml:"stages"`
	Schedule    ScheduleList      `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	AllowedDAGs AllowedDAGs       `json:"allowedDAGs" yaml:"allowed_dags"`
	Agent       AgentConfig       `json:"agent,omitempty" yaml:"agent,omitempty"`
	Disabled    bool              `json:"disabled,omitempty" yaml:"disabled,omitempty"`

	legacyStringStages bool `json:"-" yaml:"-"`
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

type PendingTurnMessage struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type PendingStageTransition struct {
	RequestedStage string    `json:"requestedStage"`
	Note           string    `json:"note,omitempty"`
	RequestedBy    string    `json:"requestedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type State struct {
	State                  LifecycleState          `json:"state"`
	Instruction            string                  `json:"instruction,omitempty"`
	InstructionUpdatedAt   time.Time               `json:"instructionUpdatedAt,omitempty"`
	InstructionUpdatedBy   string                  `json:"instructionUpdatedBy,omitempty"`
	CurrentStage           string                  `json:"currentStage,omitempty"`
	StageChangedAt         time.Time               `json:"stageChangedAt,omitempty"`
	StageChangedBy         string                  `json:"stageChangedBy,omitempty"`
	StageNote              string                  `json:"stageNote,omitempty"`
	SessionID              string                  `json:"sessionId,omitempty"`
	CurrentRunRef          *exec.DAGRunRef         `json:"currentRunRef,omitempty"`
	LastRunRef             *exec.DAGRunRef         `json:"lastRunRef,omitempty"`
	CurrentCycleID         string                  `json:"currentCycleId,omitempty"`
	WaitingReason          WaitingReason           `json:"waitingReason,omitempty"`
	PendingPrompt          *Prompt                 `json:"pendingPrompt,omitempty"`
	PendingResponse        *PromptResponse         `json:"pendingResponse,omitempty"`
	PendingStageTransition *PendingStageTransition `json:"pendingStageTransition,omitempty"`
	PendingTurnMessages    []PendingTurnMessage    `json:"pendingTurnMessages,omitempty"`
	StartRequestedAt       time.Time               `json:"startRequestedAt,omitempty"`
	LastTriggeredAt        time.Time               `json:"lastTriggeredAt,omitempty"`
	LastScheduleMinute     time.Time               `json:"lastScheduleMinute,omitempty"`
	LastUpdatedAt          time.Time               `json:"lastUpdatedAt,omitempty"`
	PausedAt               time.Time               `json:"pausedAt,omitempty"`
	PausedBy               string                  `json:"pausedBy,omitempty"`
	FinishedAt             time.Time               `json:"finishedAt,omitempty"`
	LastSummary            string                  `json:"lastSummary,omitempty"`
	LastError              string                  `json:"lastError,omitempty"`
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
	Instruction   string         `json:"instruction,omitempty"`
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
	Instruction string `json:"instruction,omitempty"`
}

type RenameRequest struct {
	NewName     string `json:"newName"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

type DuplicateRequest struct {
	NewName string `json:"newName"`
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

type OperatorMessageRequest struct {
	Message     string `json:"message"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

func newInitialState(def *Definition) *State {
	stage := ""
	if def != nil && len(def.Stages) > 0 {
		stage = def.Stages[0].Name
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

func (s *StageDefinition) UnmarshalYAML(value *yaml.Node) error {
	type rawStageDefinition struct {
		Name             string      `yaml:"name"`
		AllowedDAGs      AllowedDAGs `yaml:"allowedDAGs,omitempty"`
		AllowedDAGsSnake AllowedDAGs `yaml:"allowed_dags,omitempty"`
	}

	var raw rawStageDefinition
	if err := value.Decode(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.AllowedDAGs = raw.AllowedDAGsSnake
	if len(s.AllowedDAGs.Names) == 0 && len(s.AllowedDAGs.Tags) == 0 {
		s.AllowedDAGs = raw.AllowedDAGs
	}
	return nil
}

func (d *Definition) UnmarshalYAML(value *yaml.Node) error {
	type rawDefinition struct {
		Description      string       `yaml:"description,omitempty"`
		Purpose          string       `yaml:"purpose"`
		Goal             string       `yaml:"goal"`
		Stages           yaml.Node    `yaml:"stages"`
		Schedule         ScheduleList `yaml:"schedule,omitempty"`
		AllowedDAGs      AllowedDAGs  `yaml:"allowedDAGs"`
		AllowedDAGsSnake AllowedDAGs  `yaml:"allowed_dags"`
		Agent            AgentConfig  `yaml:"agent,omitempty"`
		Disabled         bool         `yaml:"disabled,omitempty"`
	}

	var raw rawDefinition
	if err := value.Decode(&raw); err != nil {
		return err
	}

	d.Description = raw.Description
	d.Purpose = strings.TrimSpace(raw.Purpose)
	d.Goal = strings.TrimSpace(raw.Goal)
	d.normalizeGoal()
	d.Schedule = raw.Schedule
	d.AllowedDAGs = raw.AllowedDAGsSnake
	if len(d.AllowedDAGs.Names) == 0 && len(d.AllowedDAGs.Tags) == 0 {
		d.AllowedDAGs = raw.AllowedDAGs
	}
	d.Agent = raw.Agent
	d.Disabled = raw.Disabled
	d.legacyStringStages = false

	if raw.Stages.Kind == 0 {
		d.Stages = nil
		return nil
	}
	if raw.Stages.Kind != yaml.SequenceNode {
		return fmt.Errorf("stages must be a list")
	}

	stages := make([]StageDefinition, 0, len(raw.Stages.Content))
	var sawScalar bool
	var sawMapping bool
	for _, node := range raw.Stages.Content {
		switch node.Kind {
		case yaml.ScalarNode:
			sawScalar = true
			stages = append(stages, StageDefinition{Name: strings.TrimSpace(node.Value)})
		case yaml.MappingNode:
			sawMapping = true
			var stage StageDefinition
			if err := node.Decode(&stage); err != nil {
				return err
			}
			stages = append(stages, stage)
		default:
			return fmt.Errorf("stages entries must be strings or objects")
		}
	}
	if sawScalar && sawMapping {
		return fmt.Errorf("stages must use either legacy string entries or object entries, not both")
	}
	d.legacyStringStages = sawScalar
	d.Stages = stages
	return nil
}

func (d *Definition) normalizeGoal() {
	if d == nil {
		return
	}
	d.Purpose = strings.TrimSpace(d.Purpose)
	d.Goal = strings.TrimSpace(d.Goal)
	switch {
	case d.Goal == "" && d.Purpose != "":
		d.Goal = d.Purpose
	case d.Goal != "" && d.Purpose == "":
		d.Purpose = d.Goal
	}
}

func (d *Definition) StageNames() []string {
	if d == nil {
		return nil
	}
	names := make([]string, 0, len(d.Stages))
	for _, stage := range d.Stages {
		names = append(names, stage.Name)
	}
	return names
}

func (d *Definition) StageByName(name string) *StageDefinition {
	if d == nil {
		return nil
	}
	for i := range d.Stages {
		if d.Stages[i].Name == name {
			return &d.Stages[i]
		}
	}
	return nil
}
