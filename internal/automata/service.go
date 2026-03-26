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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"gopkg.in/yaml.v3"
)

var automataNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

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
	sessionStore   agent.SessionStore
	agentAPI       *agent.API
	soulStore      agent.SoulStore
	subCmdBuilder  *runtime.SubCmdBuilder
	logger         *slog.Logger
	clock          func() time.Time
	reconcileEvery time.Duration
	mu             sync.Mutex
}

type Option func(*Service)

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

func WithSubCmdBuilder(builder *runtime.SubCmdBuilder) Option {
	return func(s *Service) {
		s.subCmdBuilder = builder
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
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
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
	if err := validateName(def.Name); err != nil {
		return err
	}
	if strings.TrimSpace(def.Purpose) == "" {
		return errors.New("purpose is required")
	}
	if strings.TrimSpace(def.Goal) == "" {
		return errors.New("goal is required")
	}
	if len(def.Stages) == 0 {
		return errors.New("at least one stage is required")
	}
	seenStages := make(map[string]struct{}, len(def.Stages))
	for i, stage := range def.Stages {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			return fmt.Errorf("stage %d is empty", i+1)
		}
		if _, ok := seenStages[stage]; ok {
			return fmt.Errorf("duplicate stage %q", stage)
		}
		seenStages[stage] = struct{}{}
		def.Stages[i] = stage
	}
	if len(def.AllowedDAGs.Names) == 0 && len(def.AllowedDAGs.Tags) == 0 {
		return errors.New("allowedDAGs.names or allowedDAGs.tags is required")
	}
	for i, name := range def.AllowedDAGs.Names {
		def.AllowedDAGs.Names[i] = strings.TrimSpace(name)
		if def.AllowedDAGs.Names[i] == "" {
			return errors.New("allowedDAGs.names contains an empty entry")
		}
		if _, err := s.dagStore.GetMetadata(ctx, def.AllowedDAGs.Names[i]); err != nil {
			return fmt.Errorf("allowed DAG %q not found: %w", def.AllowedDAGs.Names[i], err)
		}
	}
	allowed, err := s.resolveAllowedDAGs(ctx, def)
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
	if err := yaml.Unmarshal([]byte(spec), &def); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
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
	if err := os.Remove(filepath.Clean(s.definitionPath(name))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.RemoveAll(filepath.Join(s.stateDir, name))
	return nil
}

func (s *Service) ensureState(ctx context.Context, def *Definition) (*State, error) {
	state, err := s.loadState(ctx, def.Name)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = newInitialState(def)
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	if state.CurrentStage == "" || !slices.Contains(def.Stages, state.CurrentStage) {
		state.CurrentStage = def.Stages[0]
		state.StageChangedAt = s.clock()
		state.StageChangedBy = "system"
		if err := s.saveState(ctx, def.Name, state); err != nil {
			return nil, err
		}
	}
	if state.State == "" {
		state.State = StateIdle
	}
	return state, nil
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
		result = append(result, Summary{
			Name:          def.Name,
			Description:   def.Description,
			Purpose:       def.Purpose,
			Goal:          def.Goal,
			State:         state.State,
			Stage:         state.CurrentStage,
			Disabled:      def.Disabled,
			CurrentRun:    currentRun,
			LastUpdatedAt: state.LastUpdatedAt,
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
	allowed, err := s.resolveAllowedDAGs(ctx, def)
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

func (s *Service) resolveAllowedDAGs(ctx context.Context, def *Definition) ([]AllowedDAGInfo, error) {
	seen := make(map[string]AllowedDAGInfo)
	for _, name := range def.AllowedDAGs.Names {
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
	if len(def.AllowedDAGs.Tags) > 0 {
		pg := exec.NewPaginator(1, math.MaxInt)
		result, _, err := s.dagStore.List(ctx, exec.ListDAGsOptions{
			Paginator: &pg,
			Tags:      def.AllowedDAGs.Tags,
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
	state.StartRequestedAt = s.clock()
	if req.RequestedBy != "" {
		state.StageChangedBy = req.RequestedBy
	}
	if state.State == StateFinished {
		state.State = StateIdle
		state.FinishedAt = time.Time{}
	}
	return s.saveState(ctx, name, state)
}

func (s *Service) OverrideStage(ctx context.Context, name string, req StageOverrideRequest) error {
	def, err := s.GetDefinition(ctx, name)
	if err != nil {
		return err
	}
	if !slices.Contains(def.Stages, req.Stage) {
		return fmt.Errorf("unknown stage %q", req.Stage)
	}
	state, err := s.ensureState(ctx, def)
	if err != nil {
		return err
	}
	state.CurrentStage = req.Stage
	state.StageNote = req.Note
	state.StageChangedAt = s.clock()
	state.StageChangedBy = req.RequestedBy
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
	state.PendingResponse = &PromptResponse{
		PromptID:          req.PromptID,
		SelectedOptionIDs: append([]string(nil), req.SelectedOptionIDs...),
		FreeTextResponse:  req.FreeTextResponse,
		RespondedAt:       s.clock(),
	}
	state.State = StateRunning
	state.WaitingReason = WaitingReasonNone
	return s.saveState(ctx, name, state)
}

func (s *Service) systemUser(name string) agent.UserIdentity {
	return agent.UserIdentity{
		UserID:   "__automata__:" + name,
		Username: "automata/" + name,
		Role:     auth.RoleAdmin,
	}
}
