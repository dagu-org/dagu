// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/eventstore"
	"github.com/google/uuid"
)

var automataNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_]*$`)

const (
	definitionFilePerm = 0o600
	stateFilePerm      = 0o600
	dirPerm            = 0o750
)

type Service struct {
	cfg            *config.Config
	definitionsDir string
	stateDir       string
	dagStore       exec.DAGStore
	dagRunStore    exec.DAGRunStore
	dagRunCtrl     dagRunController
	coordinatorCli coordinatorCanceler
	sessionStore   agent.SessionStore
	agentAPI       *agent.API
	soulStore      agent.SoulStore
	subCmdBuilder  *runtime.SubCmdBuilder
	eventService   *eventstore.Service
	eventSource    eventstore.Source
	logger         *slog.Logger
	clock          func() time.Time
	reconcileEvery time.Duration
	mu             sync.Mutex
}

type Option func(*Service)

type dagRunController interface {
	Stop(ctx context.Context, dag *core.DAG, dagRunID string) error
}

type coordinatorCanceler interface {
	RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) error
}

type sessionUserReassigner interface {
	ReassignSessionUser(ctx context.Context, sessionID, userID string) error
}

func WithAgentAPI(api *agent.API) Option {
	return func(s *Service) {
		s.agentAPI = api
	}
}

func WithSoulStore(store agent.SoulStore) Option {
	return func(s *Service) {
		s.soulStore = store
	}
}

func WithSessionStore(store agent.SessionStore) Option {
	return func(s *Service) {
		s.sessionStore = store
	}
}

func WithDAGRunController(ctrl dagRunController) Option {
	return func(s *Service) {
		s.dagRunCtrl = ctrl
	}
}

func WithCoordinatorClient(cli coordinator.Client) Option {
	return func(s *Service) {
		s.coordinatorCli = cli
	}
}

func WithSubCmdBuilder(builder *runtime.SubCmdBuilder) Option {
	return func(s *Service) {
		s.subCmdBuilder = builder
	}
}

func WithEventService(service *eventstore.Service) Option {
	return func(s *Service) {
		s.eventService = service
	}
}

func WithEventSource(source eventstore.Source) Option {
	return func(s *Service) {
		s.eventSource = source
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func New(cfg *config.Config, dagStore exec.DAGStore, dagRunStore exec.DAGRunStore, opts ...Option) *Service {
	svc := &Service{
		cfg:            cfg,
		definitionsDir: filepath.Join(cfg.Paths.DAGsDir, "automata"),
		stateDir:       filepath.Join(cfg.Paths.DataDir, "automata"),
		dagStore:       dagStore,
		dagRunStore:    dagRunStore,
		logger:         slog.Default(),
		clock:          time.Now,
		reconcileEvery: 2 * time.Second,
		eventSource: eventstore.Source{
			Service: eventstore.SourceServiceUnknown,
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func validateName(name string) error {
	if !automataNamePattern.MatchString(name) {
		return fmt.Errorf("invalid automata name %q", name)
	}
	return nil
}

func (s *Service) definitionPath(name string) string {
	return filepath.Join(s.definitionsDir, name+".yaml")
}

func (s *Service) statePath(name string) string {
	return filepath.Join(s.stateDir, name, "state.json")
}

func (s *Service) loadState(_ context.Context, name string) (*State, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	path := s.statePath(name)
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	return &state, nil
}

func (s *Service) saveState(_ context.Context, name string, state *State) error {
	if err := validateName(name); err != nil {
		return err
	}
	if state == nil {
		return errors.New("state is required")
	}
	state.LastUpdatedAt = s.clock()
	path := s.statePath(name)
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	return fileutil.WriteJSONAtomic(path, state, stateFilePerm)
}

func (s *Service) loadDefinitionFile(ctx context.Context, path string) (*Definition, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var def Definition
	if err := parseDefinitionYAML(data, &def); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	def.Name = name
	if err := s.validateDefinition(ctx, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

func (s *Service) validateDefinition(ctx context.Context, def *Definition) error {
	if def == nil {
		return errors.New("definition is required")
	}
	def.normalizeGoal()
	if err := validateAutomataKind(def.Kind); err != nil {
		return err
	}
	if err := normalizeTags(&def.Tags); err != nil {
		return err
	}
	if err := validateName(def.Name); err != nil {
		return err
	}
	if strings.TrimSpace(def.Goal) == "" {
		return errors.New("goal is required")
	}
	if !hasAllowedDAGs(def.AllowedDAGs) {
		return errors.New("allowed_dags.names or allowed_dags.tags is required")
	}
	if err := s.normalizeAllowedDAGs(ctx, &def.AllowedDAGs); err != nil {
		return err
	}
	allowed, err := s.resolveAllowedDAGSet(ctx, def.AllowedDAGs)
	if err != nil {
		return err
	}
	if len(allowed) == 0 {
		return errors.New("definition does not resolve to any allowed DAGs")
	}
	return nil
}

func (s *Service) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	if err := os.MkdirAll(s.definitionsDir, dirPerm); err != nil {
		return nil, fmt.Errorf("create definitions dir: %w", err)
	}
	entries, err := os.ReadDir(s.definitionsDir)
	if err != nil {
		return nil, fmt.Errorf("read definitions dir: %w", err)
	}
	defs := make([]*Definition, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			continue
		}
		def, err := s.loadDefinitionFile(ctx, filepath.Join(s.definitionsDir, entry.Name()))
		if err != nil {
			s.logger.Warn("skipping invalid automata definition", "file", entry.Name(), "error", err)
			continue
		}
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs, nil
}

func (s *Service) GetDefinition(ctx context.Context, name string) (*Definition, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	def, err := s.loadDefinitionFile(ctx, s.definitionPath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, exec.ErrDAGNotFound
		}
		return nil, err
	}
	return def, nil
}

func (s *Service) GetSpec(_ context.Context, name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Clean(s.definitionPath(name)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Service) PutSpec(ctx context.Context, name, spec string) error {
	if err := validateName(name); err != nil {
		return err
	}
	var def Definition
	if err := parseDefinitionYAML([]byte(spec), &def); err != nil {
		return err
	}
	def.Name = name
	if err := s.validateDefinition(ctx, &def); err != nil {
		return err
	}
	if err := os.MkdirAll(s.definitionsDir, dirPerm); err != nil {
		return fmt.Errorf("create definitions dir: %w", err)
	}
	return fileutil.WriteFileAtomic(s.definitionPath(name), []byte(spec), definitionFilePerm)
}

func (s *Service) Delete(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := s.cleanupRuntime(ctx, name, true); err != nil {
		return err
	}
	if err := os.Remove(filepath.Clean(s.definitionPath(name))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.RemoveAll(filepath.Join(s.stateDir, name))
	return nil
}

func (s *Service) Rename(ctx context.Context, name string, req RenameRequest) error {
	if err := validateName(name); err != nil {
		return err
	}
	newName := strings.TrimSpace(req.NewName)
	if err := validateName(newName); err != nil {
		return err
	}
	if name == newName {
		return errors.New("new automata name must be different")
	}
	if err := s.assertAutomataTargetAvailable(newName); err != nil {
		return err
	}

	spec, err := s.GetSpec(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.loadState(ctx, name)
	if err != nil {
		return err
	}

	if err := fileutil.WriteFileAtomic(s.definitionPath(newName), []byte(spec), definitionFilePerm); err != nil {
		return err
	}

	rollbackNewSpec := true
	defer func() {
		if rollbackNewSpec {
			_ = os.Remove(filepath.Clean(s.definitionPath(newName)))
		}
	}()

	if state != nil && state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
			return err
		}
		if err := s.reassignSessionUser(ctx, state.SessionID, newName); err != nil {
			return err
		}
	}

	oldStateDir := filepath.Join(s.stateDir, name)
	newStateDir := filepath.Join(s.stateDir, newName)
	movedState := false
	if _, err := os.Stat(oldStateDir); err == nil {
		if err := os.Rename(oldStateDir, newStateDir); err != nil {
			return err
		}
		movedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Remove(filepath.Clean(s.definitionPath(name))); err != nil {
		if movedState {
			_ = os.Rename(newStateDir, oldStateDir)
		}
		return err
	}

	rollbackNewSpec = false
	return nil
}

func (s *Service) Duplicate(ctx context.Context, name string, req DuplicateRequest) error {
	if err := validateName(name); err != nil {
		return err
	}
	newName := strings.TrimSpace(req.NewName)
	if err := validateName(newName); err != nil {
		return err
	}
	if name == newName {
		return errors.New("duplicate automata name must be different")
	}
	spec, err := s.GetSpec(ctx, name)
	if err != nil {
		return err
	}
	if err := s.assertAutomataTargetAvailable(newName); err != nil {
		return err
	}
	return s.PutSpec(ctx, newName, spec)
}

func (s *Service) ResetState(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if _, err := s.GetDefinition(ctx, name); err != nil {
		return err
	}
	if err := s.cleanupRuntime(ctx, name, true); err != nil {
		return err
	}
	state, err := s.loadState(ctx, name)
	if err != nil {
		return err
	}
	if state == nil {
		state = newInitialState()
	} else {
		resetRuntimeState(state)
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) ensureState(ctx context.Context, def *Definition) (*State, error) {
	state, err := s.loadState(ctx, def.Name)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = newInitialState()
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	changed, err := normalizePersistedTasks(&state.Tasks)
	if err != nil {
		return nil, err
	}
	if state.State == "" {
		state.State = StateIdle
		changed = true
	}
	if state.Tasks == nil {
		state.Tasks = []Task{}
		changed = true
	}
	if isService(def) {
		if state.State == StateFinished {
			state.State = StateIdle
			state.FinishedAt = time.Time{}
			changed = true
		}
		if state.ActivatedAt.IsZero() &&
			(state.State == StateRunning || state.State == StateWaiting || state.State == StatePaused) {
			state.ActivatedAt = firstNonZeroTime(state.StartRequestedAt, state.LastUpdatedAt, s.clock())
			changed = true
		}
	} else {
		if !state.ActivatedAt.IsZero() || state.ActivatedBy != "" {
			state.ActivatedAt = time.Time{}
			state.ActivatedBy = ""
			changed = true
		}
	}
	if changed {
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (s *Service) cleanupRuntime(ctx context.Context, name string, deleteSession bool) error {
	state, err := s.loadState(ctx, name)
	if err != nil || state == nil {
		return err
	}
	if err := s.cancelTrackedChildRun(ctx, state.CurrentRunRef); err != nil {
		return err
	}
	if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
		return err
	}
	if deleteSession && state.SessionID != "" && s.sessionStore != nil {
		if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) cancelTrackedChildRun(ctx context.Context, ref *exec.DAGRunRef) error {
	if ref == nil {
		return nil
	}
	status, err := s.lookupRunStatus(ctx, ref)
	if err != nil {
		return err
	}
	if status == nil {
		return nil
	}
	if !status.Status.IsActive() && !status.Status.IsWaiting() {
		return nil
	}
	return s.requestChildRunCancel(ctx, ref)
}

func (s *Service) cancelAutomataSession(ctx context.Context, name, sessionID string) error {
	if sessionID == "" || s.agentAPI == nil {
		return nil
	}
	err := s.agentAPI.CancelSession(ctx, sessionID, s.systemUser(name).UserID)
	if err == nil || errors.Is(err, agent.ErrSessionNotFound) {
		return nil
	}
	return err
}

func (s *Service) reassignSessionUser(ctx context.Context, sessionID, newName string) error {
	if sessionID == "" || s.sessionStore == nil {
		return nil
	}
	if reassigner, ok := s.sessionStore.(sessionUserReassigner); ok {
		return reassigner.ReassignSessionUser(ctx, sessionID, s.systemUser(newName).UserID)
	}
	return errors.New("session store does not support automata rename with transcript preservation")
}

func (s *Service) assertAutomataTargetAvailable(name string) error {
	if _, err := os.Stat(filepath.Clean(s.definitionPath(name))); err == nil {
		return fmt.Errorf("automata %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, name)); err == nil {
		return fmt.Errorf("automata %q already has existing runtime state", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	defs, err := s.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Summary, 0, len(defs))
	for _, def := range defs {
		state, err := s.ensureState(ctx, def)
		if err != nil {
			return nil, err
		}
		currentRun, _ := s.currentRunSummary(ctx, state)
		view := DeriveView(def, state)
		result = append(result, Summary{
			Name:                def.Name,
			Kind:                def.Kind,
			Description:         def.Description,
			Purpose:             def.Purpose,
			Goal:                def.Goal,
			Tags:                append([]string(nil), def.Tags...),
			Instruction:         state.Instruction,
			State:               state.State,
			DisplayStatus:       view.DisplayStatus,
			Busy:                view.Busy,
			NeedsInput:          view.NeedsInput,
			Disabled:            def.Disabled,
			CurrentRun:          currentRun,
			OpenTaskCount:       countTasksByState(state.Tasks, TaskStateOpen),
			DoneTaskCount:       countTasksByState(state.Tasks, TaskStateDone),
			NextTaskDescription: nextOpenTaskDescription(state.Tasks),
			LastUpdatedAt:       state.LastUpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) Detail(ctx context.Context, name string) (*Detail, error) {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return nil, err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return nil, err
	}
	allowed, err := s.resolveAllowedDAGSet(ctx, def.AllowedDAGs)
	if err != nil {
		return nil, err
	}
	currentRun, _ := s.currentRunSummary(ctx, state)
	recentRuns, _ := s.recentRuns(ctx, name)
	var messages []agent.Message
	if s.sessionStore != nil && state.SessionID != "" {
		msgs, err := s.sessionStore.GetMessages(ctx, state.SessionID)
		if err == nil {
			messages = msgs
		}
	}
	return &Detail{
		Definition:  def,
		State:       state,
		AllowedDAGs: allowed,
		CurrentRun:  currentRun,
		RecentRuns:  recentRuns,
		Messages:    messages,
	}, nil
}

func (s *Service) resolveAllowedDAGSet(ctx context.Context, allowed AllowedDAGs) ([]AllowedDAGInfo, error) {
	seen := make(map[string]AllowedDAGInfo)
	for _, name := range allowed.Names {
		dag, err := s.dagStore.GetMetadata(ctx, name)
		if err != nil {
			return nil, err
		}
		seen[dag.Name] = AllowedDAGInfo{
			Name:        dag.Name,
			Description: dag.Description,
			Tags:        dag.Tags.Strings(),
		}
	}
	if len(allowed.Tags) > 0 {
		pg := exec.NewPaginator(1, math.MaxInt)
		result, _, err := s.dagStore.List(ctx, exec.ListDAGsOptions{
			Paginator: &pg,
			Tags:      allowed.Tags,
		})
		if err != nil {
			return nil, err
		}
		for _, dag := range result.Items {
			seen[dag.Name] = AllowedDAGInfo{
				Name:        dag.Name,
				Description: dag.Description,
				Tags:        dag.Tags.Strings(),
			}
		}
	}
	out := make([]AllowedDAGInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) currentRunSummary(ctx context.Context, state *State) (*RunSummary, error) {
	if state == nil || state.CurrentRunRef == nil {
		return nil, nil
	}
	statuses, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithFrom(exec.NewUTC(time.Unix(0, 0))),
		exec.WithExactName(state.CurrentRunRef.Name),
		exec.WithDAGRunID(state.CurrentRunRef.ID),
		exec.WithLimit(1),
	)
	if err != nil || len(statuses) == 0 {
		return nil, err
	}
	return toRunSummary(statuses[0]), nil
}

func (s *Service) recentRuns(ctx context.Context, name string) ([]RunSummary, error) {
	from := exec.NewUTC(s.clock().Add(-30 * 24 * time.Hour))
	statuses, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithFrom(from),
		exec.WithTags([]string{"automata=" + strings.ToLower(name)}),
		exec.WithLimit(20),
	)
	if err != nil {
		return nil, err
	}
	out := make([]RunSummary, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, *toRunSummary(status))
	}
	return out, nil
}

func (s *Service) HandleScheduleTick(ctx context.Context, tickTime time.Time) error {
	defs, err := s.ListDefinitions(ctx)
	if err != nil {
		return err
	}
	tickTime = tickTime.Truncate(time.Minute)
	for _, def := range defs {
		if err := s.handleScheduledServiceTick(ctx, def, tickTime); err != nil {
			s.logger.Warn("automata schedule tick failed",
				"automata", def.Name,
				"error", err,
			)
		}
	}
	return nil
}

func (s *Service) handleScheduledServiceTick(ctx context.Context, def *Definition, tickTime time.Time) error {
	if def == nil || def.Disabled || !isService(def) || len(def.Schedule) == 0 {
		return nil
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if !isServiceActivated(state) || state.State == StatePaused {
		return nil
	}
	if strings.TrimSpace(state.Instruction) == "" || !hasOpenTask(state.Tasks) {
		return nil
	}
	if state.PendingPrompt != nil || state.CurrentRunRef != nil || len(state.PendingTurnMessages) > 0 {
		return nil
	}
	if !state.LastScheduleMinute.IsZero() && state.LastScheduleMinute.Equal(tickTime) {
		return nil
	}
	if !scheduleListDueAt(def.Schedule, tickTime) {
		return nil
	}
	activity := s.inspectSessionActivity(ctx, def.Name, state)
	if activity.Working || activity.HasPendingPrompt || activity.HasQueuedInput {
		return nil
	}

	queueTurnMessage(state, "scheduled_tick", s.buildScheduledTickMessage(def, state, tickTime), s.clock())
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	state.LastScheduleMinute = tickTime
	return s.saveState(ctx, def.Name, state)
}

func scheduleListDueAt(items ScheduleList, tickTime time.Time) bool {
	for _, item := range items {
		if _, due := item.DueAt(tickTime); due {
			return true
		}
	}
	return false
}

func toRunSummary(status *exec.DAGRunStatus) *RunSummary {
	if status == nil {
		return nil
	}
	return &RunSummary{
		Name:        status.Name,
		DAGRunID:    status.DAGRunID,
		Status:      status.Status.String(),
		TriggerType: status.TriggerType.String(),
		StartedAt:   status.StartedAt,
		FinishedAt:  status.FinishedAt,
		CreatedAt:   time.UnixMilli(status.CreatedAt),
		Error:       status.Error,
	}
}

func (s *Service) RequestStart(ctx context.Context, name string, req StartRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	if def.Disabled {
		return errors.New("automata is disabled")
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if isService(def) {
		return s.requestServiceStart(ctx, def, state, req)
	}
	if state.State == StateRunning || state.State == StateWaiting || state.State == StatePaused {
		return errors.New("automata already has an active task")
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		instruction = strings.TrimSpace(state.Instruction)
	}
	if instruction == "" {
		return errors.New("instruction is required before starting automata")
	}
	if !hasOpenTask(state.Tasks) {
		return errors.New("at least one open task is required before starting automata")
	}
	if state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
			return err
		}
		if s.sessionStore != nil {
			if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
				return err
			}
		}
	}
	resetRuntimeState(state)
	state.State = StateRunning
	state.Instruction = instruction
	state.InstructionUpdatedAt = s.clock()
	state.InstructionUpdatedBy = req.RequestedBy
	state.StartRequestedAt = s.clock()
	queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
	return s.saveState(ctx, name, state)
}

func (s *Service) requestServiceStart(ctx context.Context, def *Definition, state *State, req StartRequest) error {
	if isServiceActivated(state) {
		return errors.New("service automata is already active")
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		instruction = strings.TrimSpace(state.Instruction)
	}
	if instruction == "" {
		return errors.New("instruction is required before starting automata")
	}
	if state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, def.Name, state.SessionID); err != nil {
			return err
		}
		if s.sessionStore != nil {
			if err := s.sessionStore.DeleteSession(ctx, state.SessionID); err != nil && !errors.Is(err, agent.ErrSessionNotFound) {
				return err
			}
		}
	}
	resetRuntimeState(state)
	state.State = StateIdle
	state.Instruction = instruction
	state.InstructionUpdatedAt = s.clock()
	state.InstructionUpdatedBy = req.RequestedBy
	state.ActivatedAt = s.clock()
	state.ActivatedBy = req.RequestedBy
	state.WaitingReason = WaitingReasonNone
	if hasOpenTask(state.Tasks) {
		state.State = StateRunning
		state.StartRequestedAt = s.clock()
		queueTurnMessage(state, "kickoff", s.buildKickoffMessage(def, state), s.clock())
	}
	return s.saveState(ctx, def.Name, state)
}

func (s *Service) Pause(ctx context.Context, name, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.State == StatePaused {
		return errors.New("automata is already paused")
	}
	if isService(def) {
		if !isServiceActivated(state) {
			return errors.New("service automata is not active")
		}
	} else if state.State != StateRunning && state.State != StateWaiting {
		return errors.New("only active automata can be paused")
	}
	state.PausedFromState = state.State
	state.State = StatePaused
	state.WaitingReason = WaitingReasonNone
	state.PausedAt = s.clock()
	state.PausedBy = requestedBy
	return s.saveState(ctx, name, state)
}

func (s *Service) Resume(ctx context.Context, name, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.State != StatePaused {
		return errors.New("automata is not paused")
	}
	pausedFromState := state.PausedFromState
	state.PausedAt = time.Time{}
	state.PausedBy = ""
	state.PausedFromState = ""
	if state.PendingPrompt != nil {
		state.State = StateWaiting
		state.WaitingReason = WaitingReasonHuman
		return s.saveState(ctx, name, state)
	}
	if isService(def) && pausedFromState == StateIdle &&
		state.CurrentRunRef == nil && len(state.PendingTurnMessages) == 0 {
		state.State = StateIdle
		state.WaitingReason = WaitingReasonNone
		return s.saveState(ctx, name, state)
	}
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	if state.CurrentRunRef == nil && len(state.PendingTurnMessages) == 0 {
		queueTurnMessage(state, "resume", s.buildResumeMessage(def, state, requestedBy), s.clock())
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) SubmitHumanResponse(ctx context.Context, name string, req HumanResponseRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if state.PendingPrompt == nil {
		return errors.New("automata is not waiting for human input")
	}
	if req.PromptID == "" || req.PromptID != state.PendingPrompt.ID {
		return errors.New("prompt ID does not match the pending prompt")
	}
	response := &PromptResponse{
		PromptID:          req.PromptID,
		SelectedOptionIDs: append([]string(nil), req.SelectedOptionIDs...),
		FreeTextResponse:  req.FreeTextResponse,
		RespondedAt:       s.clock(),
	}
	paused := state.State == StatePaused
	queueTurnMessage(state, "human_response", s.buildHumanResponseMessage(state.PendingPrompt, response), s.clock())
	state.PendingPrompt = nil
	state.PendingResponse = nil
	if paused {
		state.WaitingReason = WaitingReasonNone
	} else {
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) CreateTask(ctx context.Context, name string, req CreateTaskRequest) (*Task, error) {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return nil, err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return nil, err
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		return nil, errors.New("task description is required")
	}
	now := s.clock()
	task := Task{
		ID:          uuid.NewString(),
		Description: description,
		State:       TaskStateOpen,
		CreatedAt:   now,
		CreatedBy:   req.RequestedBy,
		UpdatedAt:   now,
		UpdatedBy:   req.RequestedBy,
	}
	state.Tasks = append(state.Tasks, task)
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "created")
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Service) UpdateTask(ctx context.Context, name, taskID string, req UpdateTaskRequest) (*Task, error) {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return nil, err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return nil, err
	}
	idx := findTaskIndex(state.Tasks, taskID)
	if idx < 0 {
		return nil, fmt.Errorf("unknown task %q", taskID)
	}
	if req.Description == nil && req.Done == nil {
		return nil, errors.New("no task changes requested")
	}
	now := s.clock()
	task := &state.Tasks[idx]
	changed := false
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if description == "" {
			return nil, errors.New("task description is required")
		}
		if description != task.Description {
			task.Description = description
			changed = true
		}
	}
	if req.Done != nil {
		targetState := TaskStateOpen
		if *req.Done {
			targetState = TaskStateDone
		}
		if task.State != targetState {
			task.State = targetState
			if targetState == TaskStateDone {
				task.DoneAt = now
				task.DoneBy = req.RequestedBy
			} else {
				task.DoneAt = time.Time{}
				task.DoneBy = ""
			}
			changed = true
		}
	}
	if !changed {
		copied := *task
		return &copied, nil
	}
	task.UpdatedAt = now
	task.UpdatedBy = req.RequestedBy
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "updated")
	if err := s.saveState(ctx, name, state); err != nil {
		return nil, err
	}
	copied := *task
	return &copied, nil
}

func (s *Service) SetTaskDone(ctx context.Context, name, taskID string, done bool, requestedBy string) (*Task, error) {
	return s.UpdateTask(ctx, name, taskID, UpdateTaskRequest{
		Done:        &done,
		RequestedBy: requestedBy,
	})
}

func (s *Service) DeleteTask(ctx context.Context, name, taskID, requestedBy string) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	idx := findTaskIndex(state.Tasks, taskID)
	if idx < 0 {
		return fmt.Errorf("unknown task %q", taskID)
	}
	state.Tasks = append(state.Tasks[:idx], state.Tasks[idx+1:]...)
	s.queueTaskListUpdate(ctx, name, state, requestedBy, "deleted")
	return s.saveState(ctx, name, state)
}

func (s *Service) ReorderTasks(ctx context.Context, name string, req ReorderTasksRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if len(req.TaskIDs) != len(state.Tasks) {
		return errors.New("taskIds must contain every task exactly once")
	}
	existing := make(map[string]Task, len(state.Tasks))
	for _, task := range state.Tasks {
		existing[task.ID] = task
	}
	reordered := make([]Task, 0, len(req.TaskIDs))
	seen := make(map[string]struct{}, len(req.TaskIDs))
	for _, taskID := range req.TaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return errors.New("taskIds contains an empty entry")
		}
		if _, ok := seen[taskID]; ok {
			return fmt.Errorf("duplicate task id %q", taskID)
		}
		task, ok := existing[taskID]
		if !ok {
			return fmt.Errorf("unknown task %q", taskID)
		}
		seen[taskID] = struct{}{}
		reordered = append(reordered, task)
	}
	state.Tasks = reordered
	s.queueTaskListUpdate(ctx, name, state, req.RequestedBy, "reordered")
	return s.saveState(ctx, name, state)
}

func (s *Service) SubmitOperatorMessage(ctx context.Context, name string, req OperatorMessageRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	if def.Disabled {
		return errors.New("automata is disabled")
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return errors.New("message is required")
	}
	if isService(def) {
		if !isServiceActivated(state) {
			return errors.New("service automata is not active")
		}
		if state.State == StatePaused {
			return errors.New("service automata is paused")
		}
	} else if state.State != StateRunning && state.State != StateWaiting && state.State != StatePaused {
		return errors.New("automata is not running an active task")
	}
	if state.PendingPrompt != nil {
		return errors.New("respond to the pending prompt before sending a general operator message")
	}
	operatorMessage := buildOperatorMessage(req.RequestedBy, message)
	turnMessage := operatorMessage
	if state.CurrentRunRef != nil || (!isService(def) && state.State == StatePaused) {
		if err := s.appendOperatorMessageToSession(ctx, name, state, operatorMessage); err != nil {
			s.logger.Warn("failed to append queued operator message to automata session",
				"automata", name,
				"session_id", state.SessionID,
				"error", err,
			)
		} else {
			turnMessage = buildOperatorWakeMessage()
		}
	}
	queueTurnMessage(state, "operator_message", turnMessage, s.clock())
	if state.State != StatePaused && state.CurrentRunRef == nil {
		state.State = StateRunning
		state.WaitingReason = WaitingReasonNone
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) queueTaskListUpdate(ctx context.Context, name string, state *State, requestedBy, action string) {
	if state == nil {
		return
	}
	if state.State != StateRunning && state.State != StateWaiting && state.State != StatePaused {
		return
	}
	updateMessage := buildTaskListUpdateMessage(requestedBy, action, state.Tasks)
	turnMessage := updateMessage
	if state.SessionID != "" && (state.CurrentRunRef != nil || state.State == StatePaused || state.PendingPrompt != nil) {
		if err := s.appendOperatorMessageToSession(ctx, name, state, updateMessage); err != nil {
			s.logger.Warn("failed to append task list update to automata session",
				"automata", name,
				"session_id", state.SessionID,
				"error", err,
			)
		} else {
			turnMessage = buildOperatorWakeMessage()
		}
	}
	queueTurnMessage(state, "task_list_updated", turnMessage, s.clock())
}

func (s *Service) appendOperatorMessageToSession(ctx context.Context, name string, state *State, content string) error {
	if state == nil || state.SessionID == "" {
		return errors.New("session is not initialized")
	}
	msg := agent.Message{
		ID:        uuid.NewString(),
		SessionID: state.SessionID,
		Type:      agent.MessageTypeUser,
		Content:   content,
		CreatedAt: s.clock(),
		LLMData: &llm.Message{
			Role:    llm.RoleUser,
			Content: content,
		},
	}
	if s.agentAPI != nil {
		if _, err := s.agentAPI.AppendExternalMessage(ctx, state.SessionID, s.systemUser(name), msg); err == nil {
			return nil
		} else if !errors.Is(err, agent.ErrSessionNotFound) {
			return err
		}
	}
	if s.sessionStore == nil {
		return errors.New("session store is not configured")
	}
	seqID, err := s.sessionStore.GetLatestSequenceID(ctx, state.SessionID)
	if err != nil {
		return err
	}
	msg.SequenceID = seqID + 1
	return s.sessionStore.AddMessage(ctx, state.SessionID, &msg)
}

func (s *Service) systemUser(name string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   "__automata__:" + name,
		Username: "automata/" + name,
		Role:     auth.RoleAdmin,
	}
}

func resetRuntimeState(state *State) {
	if state == nil {
		return
	}
	state.WaitingReason = WaitingReasonNone
	state.PendingPrompt = nil
	state.PendingResponse = nil
	state.PendingTurnMessages = nil
	state.SessionID = ""
	state.CurrentRunRef = nil
	state.LastRunRef = nil
	state.CurrentCycleID = ""
	state.StartRequestedAt = time.Time{}
	state.PausedAt = time.Time{}
	state.PausedBy = ""
	state.PausedFromState = ""
	state.ActivatedAt = time.Time{}
	state.ActivatedBy = ""
	state.FinishedAt = time.Time{}
	state.LastScheduleMinute = time.Time{}
	state.LastSummary = ""
	state.LastError = ""
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func (s *Service) normalizeAllowedDAGs(ctx context.Context, allowed *AllowedDAGs) error {
	if allowed == nil {
		return nil
	}
	for i, name := range allowed.Names {
		allowed.Names[i] = strings.TrimSpace(name)
		if allowed.Names[i] == "" {
			return errors.New("allowed_dags.names contains an empty entry")
		}
		if _, err := s.dagStore.GetMetadata(ctx, allowed.Names[i]); err != nil {
			return fmt.Errorf("allowed DAG %q not found: %w", allowed.Names[i], err)
		}
	}
	for i, tag := range allowed.Tags {
		allowed.Tags[i] = strings.TrimSpace(tag)
		if allowed.Tags[i] == "" {
			return errors.New("allowed_dags.tags contains an empty entry")
		}
	}
	return nil
}

func hasAllowedDAGs(allowed AllowedDAGs) bool {
	return len(allowed.Names) > 0 || len(allowed.Tags) > 0
}

func normalizeTags(tags *[]string) error {
	if tags == nil {
		return nil
	}
	parsed := core.NewTags(*tags)
	if err := core.ValidateTags(parsed); err != nil {
		return fmt.Errorf("invalid tags: %w", err)
	}
	*tags = parsed.Strings()
	return nil
}

func (s *Service) requestChildRunCancel(ctx context.Context, ref *exec.DAGRunRef) error {
	if ref == nil {
		return nil
	}
	status, _ := s.lookupRunStatus(ctx, ref)
	if status != nil && status.WorkerID != "" && s.coordinatorCli != nil {
		return s.coordinatorCli.RequestCancel(ctx, ref.Name, ref.ID, nil)
	}
	if s.dagRunCtrl != nil {
		dag, err := s.dagStore.GetDetails(ctx, ref.Name)
		if err != nil {
			return err
		}
		return s.dagRunCtrl.Stop(ctx, dag, ref.ID)
	}
	attempt, err := s.dagRunStore.FindAttempt(ctx, *ref)
	if err == nil {
		return attempt.Abort(ctx)
	}
	return errors.New("dag run control is not configured")
}

func (s *Service) lookupRunStatus(ctx context.Context, ref *exec.DAGRunRef) (*exec.DAGRunStatus, error) {
	if ref == nil {
		return nil, nil
	}
	statuses, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithFrom(exec.NewUTC(time.Unix(0, 0))),
		exec.WithExactName(ref.Name),
		exec.WithDAGRunID(ref.ID),
		exec.WithLimit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	return statuses[0], nil
}

func queueTurnMessage(state *State, kind, message string, now time.Time) {
	if state == nil || strings.TrimSpace(message) == "" {
		return
	}
	state.PendingTurnMessages = append(state.PendingTurnMessages, PendingTurnMessage{
		ID:        uuid.NewString(),
		Kind:      kind,
		Message:   message,
		CreatedAt: now,
	})
}

func buildOperatorMessage(requestedBy, message string) string {
	if requestedBy == "" {
		return "Operator update:\n" + message
	}
	return fmt.Sprintf("Operator update from %s:\n%s", requestedBy, message)
}

func buildTaskListUpdateMessage(requestedBy, action string, tasks []Task) string {
	prefix := "Operator updated the checklist"
	if requestedBy != "" {
		prefix = fmt.Sprintf("Operator %s updated the checklist", requestedBy)
	}
	if strings.TrimSpace(action) != "" {
		prefix = fmt.Sprintf("%s (%s)", prefix, action)
	}
	return fmt.Sprintf("%s.\nCurrent checklist:\n%s", prefix, buildTaskListSummary(tasks))
}

func buildOperatorWakeMessage() string {
	return "A new operator update was appended to the session while execution was blocked. Review the latest user message(s) and continue the task when you can act again."
}
