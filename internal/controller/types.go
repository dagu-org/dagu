// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
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

type ControllerKind string

const (
	ControllerKindWorkflow ControllerKind = "workflow"
	ControllerKindService  ControllerKind = "service"
)

type DisplayStatus string

const (
	DisplayStatusIdle     DisplayStatus = "idle"
	DisplayStatusRunning  DisplayStatus = "running"
	DisplayStatusPaused   DisplayStatus = "paused"
	DisplayStatusFinished DisplayStatus = "finished"
)

type Workflows struct {
	Names  []string `json:"names,omitempty" yaml:"names,omitempty"`
	Labels []string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type AgentConfig struct {
	Model    string `json:"model,omitempty" yaml:"model,omitempty"`
	Soul     string `json:"soul,omitempty" yaml:"soul,omitempty"`
	SafeMode bool   `json:"safeMode,omitempty" yaml:"safeMode,omitempty"`
}

type CronScheduleList []core.Schedule

func (s *CronScheduleList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		*s = nil
		return nil
	}

	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("trigger.schedules must be a list of cron expressions")
	}

	expressions := make([]string, 0, len(value.Content))
	for i, node := range value.Content {
		if node.Kind != yaml.ScalarNode {
			return fmt.Errorf("trigger.schedules[%d] must be a string", i)
		}
		expressions = append(expressions, strings.TrimSpace(node.Value))
	}

	if len(expressions) == 0 {
		*s = nil
		return nil
	}

	out := make([]core.Schedule, 0, len(expressions))
	for i, expr := range expressions {
		if expr == "" {
			continue
		}
		parsed, err := cron.ParseStandard(expr)
		if err != nil {
			return fmt.Errorf("invalid trigger.schedules[%d] %q: %w", i, expr, err)
		}
		out = append(out, core.Schedule{
			Expression: expr,
			Parsed:     parsed,
		})
	}
	*s = out
	return nil
}

type TriggerMode string

const (
	TriggerModeManual TriggerMode = "manual"
	TriggerModeCron   TriggerMode = "cron"
)

type Trigger struct {
	Type      TriggerMode      `json:"type" yaml:"type"`
	Schedules CronScheduleList `json:"schedules,omitempty" yaml:"schedules,omitempty"`
	Prompt    string           `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

func (t *Trigger) UnmarshalYAML(value *yaml.Node) error {
	type rawTrigger struct {
		Type      TriggerMode      `yaml:"type"`
		Schedules CronScheduleList `yaml:"schedules,omitempty"`
		Prompt    string           `yaml:"prompt,omitempty"`
	}

	var raw rawTrigger
	if err := value.Decode(&raw); err != nil {
		return err
	}

	t.Type = normalizeTriggerMode(raw.Type)
	t.Schedules = raw.Schedules
	t.Prompt = strings.TrimSpace(raw.Prompt)
	return t.Validate()
}

func (t Trigger) Validate() error {
	switch t.Type {
	case TriggerModeManual:
		if len(t.Schedules) > 0 {
			return errors.New("trigger.schedules is only valid for cron trigger")
		}
		if t.Prompt != "" {
			return errors.New("trigger.prompt is only valid for cron trigger")
		}
		return nil
	case TriggerModeCron:
		if len(t.Schedules) == 0 {
			return errors.New("trigger.schedules is required for cron trigger")
		}
		if strings.TrimSpace(t.Prompt) == "" {
			return errors.New("trigger.prompt is required for cron trigger")
		}
		return nil
	case "":
		return errors.New("trigger.type is required")
	default:
		return fmt.Errorf("invalid trigger.type %q", t.Type)
	}
}

type Definition struct {
	Name          string         `json:"name"`
	Kind          ControllerKind `json:"kind" yaml:"kind,omitempty"`
	Nickname      string         `json:"nickname,omitempty" yaml:"nickname,omitempty"`
	IconURL       string         `json:"iconUrl,omitempty" yaml:"icon_url,omitempty"`
	Description   string         `json:"description,omitempty" yaml:"description,omitempty"`
	Purpose       string         `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Goal          string         `json:"goal" yaml:"goal"`
	ClonedFrom    string         `json:"clonedFrom,omitempty" yaml:"cloned_from,omitempty"`
	ResetOnFinish bool           `json:"resetOnFinish,omitempty" yaml:"reset_on_finish,omitempty"`
	Labels        []string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Trigger       Trigger        `json:"trigger" yaml:"trigger"`
	Workflows     Workflows      `json:"workflows,omitempty" yaml:"workflows,omitempty"`
	Agent         AgentConfig    `json:"agent" yaml:"agent,omitempty"`
	Disabled      bool           `json:"disabled,omitempty" yaml:"disabled,omitempty"`
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

type TaskState string

const (
	TaskStateOpen TaskState = "open"
	TaskStateDone TaskState = "done"
)

type Task struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	State       TaskState `json:"state"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy,omitempty"`
	DoneAt      time.Time `json:"doneAt"`
	DoneBy      string    `json:"doneBy,omitempty"`
}

type TaskTemplate struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy,omitempty"`
}

type State struct {
	State                LifecycleState       `json:"state"`
	Instruction          string               `json:"instruction,omitempty"`
	InstructionUpdatedAt time.Time            `json:"instructionUpdatedAt"`
	InstructionUpdatedBy string               `json:"instructionUpdatedBy,omitempty"`
	ActivatedAt          time.Time            `json:"activatedAt"`
	ActivatedBy          string               `json:"activatedBy,omitempty"`
	TaskTemplates        []TaskTemplate       `json:"taskTemplates,omitempty"`
	Tasks                []Task               `json:"tasks,omitempty"`
	SessionID            string               `json:"sessionId,omitempty"`
	CurrentRunRef        *exec.DAGRunRef      `json:"currentRunRef,omitempty"`
	LastRunRef           *exec.DAGRunRef      `json:"lastRunRef,omitempty"`
	CurrentCycleID       string               `json:"currentCycleId,omitempty"`
	WaitingReason        WaitingReason        `json:"waitingReason,omitempty"`
	PendingPrompt        *Prompt              `json:"pendingPrompt,omitempty"`
	PendingResponse      *PromptResponse      `json:"pendingResponse,omitempty"`
	PendingTurnMessages  []PendingTurnMessage `json:"pendingTurnMessages,omitempty"`
	StartRequestedAt     time.Time            `json:"startRequestedAt"`
	LastTriggeredAt      time.Time            `json:"lastTriggeredAt"`
	LastScheduleMinute   time.Time            `json:"lastScheduleMinute"`
	LastUpdatedAt        time.Time            `json:"lastUpdatedAt"`
	PausedAt             time.Time            `json:"pausedAt"`
	PausedBy             string               `json:"pausedBy,omitempty"`
	PausedFromState      LifecycleState       `json:"pausedFromState,omitempty"`
	FinishedAt           time.Time            `json:"finishedAt"`
	LastSummary          string               `json:"lastSummary,omitempty"`
	LastError            string               `json:"lastError,omitempty"`
}

type WorkflowInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

type RunSummary struct {
	Name        string    `json:"name"`
	DAGRunID    string    `json:"dagRunId"`
	Status      string    `json:"status"`
	TriggerType string    `json:"triggerType,omitempty"`
	StartedAt   string    `json:"startedAt,omitempty"`
	FinishedAt  string    `json:"finishedAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	Error       string    `json:"error,omitempty"`
}

type Summary struct {
	Name                string         `json:"name"`
	Kind                ControllerKind `json:"kind"`
	Nickname            string         `json:"nickname,omitempty"`
	IconURL             string         `json:"iconUrl,omitempty"`
	Description         string         `json:"description,omitempty"`
	Purpose             string         `json:"purpose"`
	Goal                string         `json:"goal"`
	ClonedFrom          string         `json:"clonedFrom,omitempty"`
	ResetOnFinish       bool           `json:"resetOnFinish,omitempty"`
	Labels              []string       `json:"labels,omitempty"`
	Instruction         string         `json:"instruction,omitempty"`
	State               LifecycleState `json:"state"`
	DisplayStatus       DisplayStatus  `json:"displayStatus"`
	Busy                bool           `json:"busy"`
	NeedsInput          bool           `json:"needsInput"`
	Disabled            bool           `json:"disabled,omitempty"`
	CurrentRun          *RunSummary    `json:"currentRun,omitempty"`
	OpenTaskCount       int            `json:"openTaskCount"`
	DoneTaskCount       int            `json:"doneTaskCount"`
	NextTaskDescription string         `json:"nextTaskDescription,omitempty"`
	LastUpdatedAt       time.Time      `json:"lastUpdatedAt"`
}

type Detail struct {
	Definition         *Definition     `json:"definition"`
	State              *State          `json:"state"`
	Workflows          []WorkflowInfo  `json:"workflows"`
	ArtifactDir        string          `json:"artifactDir,omitempty"`
	ArtifactsAvailable bool            `json:"artifactsAvailable,omitempty"`
	TaskTemplates      []TaskTemplate  `json:"taskTemplates,omitempty"`
	CurrentRun         *RunSummary     `json:"currentRun,omitempty"`
	RecentRuns         []RunSummary    `json:"recentRuns,omitempty"`
	Messages           []agent.Message `json:"messages,omitempty"`
}

type Document struct {
	Name     string `json:"name"`
	Document string `json:"document"`
	Content  string `json:"content"`
	Path     string `json:"path"`
}

type Memory = Document

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

type HumanResponseRequest struct {
	PromptID          string   `json:"promptId"`
	SelectedOptionIDs []string `json:"selectedOptionIds,omitempty"`
	FreeTextResponse  string   `json:"freeTextResponse,omitempty"`
}

type OperatorMessageRequest struct {
	Message     string `json:"message"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

type CreateTaskRequest struct {
	Description string `json:"description"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

type UpdateTaskRequest struct {
	Description *string `json:"description,omitempty"`
	Done        *bool   `json:"done,omitempty"`
	RequestedBy string  `json:"requestedBy,omitempty"`
}

type ReorderTasksRequest struct {
	TaskIDs     []string `json:"taskIds"`
	RequestedBy string   `json:"requestedBy,omitempty"`
}

func newInitialState() *State {
	now := time.Now()
	return &State{
		State:         StateIdle,
		TaskTemplates: []TaskTemplate{},
		Tasks:         []Task{},
		LastUpdatedAt: now,
	}
}

func nextCycleID() string {
	return uuid.NewString()
}

func (d *Definition) UnmarshalYAML(value *yaml.Node) error {
	type rawDefinition struct {
		Kind               ControllerKind `yaml:"kind,omitempty"`
		Nickname           string         `yaml:"nickname,omitempty"`
		IconURL            string         `yaml:"iconUrl,omitempty"`
		IconURLSnake       string         `yaml:"icon_url,omitempty"`
		Description        string         `yaml:"description,omitempty"`
		Purpose            string         `yaml:"purpose"`
		Goal               string         `yaml:"goal"`
		ClonedFrom         string         `yaml:"clonedFrom,omitempty"`
		ClonedFromSnake    string         `yaml:"cloned_from,omitempty"`
		ResetOnFinish      bool           `yaml:"resetOnFinish,omitempty"`
		ResetOnFinishSnake bool           `yaml:"reset_on_finish,omitempty"`
		Labels             []string       `yaml:"labels"`
		Trigger            Trigger        `yaml:"trigger"`
		Workflows          Workflows      `yaml:"workflows,omitempty"`
		Agent              AgentConfig    `yaml:"agent,omitempty"`
		Disabled           bool           `yaml:"disabled,omitempty"`
	}

	var raw rawDefinition
	if err := value.Decode(&raw); err != nil {
		return err
	}

	d.Nickname = strings.TrimSpace(raw.Nickname)
	d.IconURL = strings.TrimSpace(raw.IconURLSnake)
	if d.IconURL == "" {
		d.IconURL = strings.TrimSpace(raw.IconURL)
	}
	d.Description = raw.Description
	d.Kind = normalizeControllerKind(raw.Kind)
	d.Purpose = strings.TrimSpace(raw.Purpose)
	d.Goal = strings.TrimSpace(raw.Goal)
	d.ClonedFrom = strings.TrimSpace(raw.ClonedFromSnake)
	if d.ClonedFrom == "" {
		d.ClonedFrom = strings.TrimSpace(raw.ClonedFrom)
	}
	d.ResetOnFinish = raw.ResetOnFinish || raw.ResetOnFinishSnake
	d.Labels = append([]string(nil), raw.Labels...)
	d.normalizeGoal()
	d.Trigger = raw.Trigger
	d.Workflows = raw.Workflows
	d.Agent = raw.Agent
	d.Disabled = raw.Disabled
	if err := d.Trigger.Validate(); err != nil {
		return err
	}
	return nil
}

func (d *Definition) normalizeGoal() {
	if d == nil {
		return
	}
	d.Kind = normalizeControllerKind(d.Kind)
	d.Purpose = strings.TrimSpace(d.Purpose)
	d.Goal = strings.TrimSpace(d.Goal)
	switch {
	case d.Goal == "" && d.Purpose != "":
		d.Goal = d.Purpose
	case d.Goal != "" && d.Purpose == "":
		d.Purpose = d.Goal
	}
}

func normalizeControllerKind(_ ControllerKind) ControllerKind {
	return ControllerKindWorkflow
}

func normalizeTriggerMode(value TriggerMode) TriggerMode {
	switch TriggerMode(strings.ToLower(strings.TrimSpace(string(value)))) {
	case TriggerModeManual:
		return TriggerModeManual
	case TriggerModeCron:
		return TriggerModeCron
	default:
		return value
	}
}
