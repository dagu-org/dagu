// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
)

func (s *Service) definitionPath(name string) string {
	return filepath.Join(s.definitionsDir, name+".yaml")
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
	def.Nickname = strings.TrimSpace(def.Nickname)
	def.IconURL = strings.TrimSpace(def.IconURL)
	def.StandingInstruction = strings.TrimSpace(def.StandingInstruction)
	if err := validateAutomataKind(def.Kind); err != nil {
		return err
	}
	if err := normalizeTags(&def.Tags); err != nil {
		return err
	}
	if err := validateName(def.Name); err != nil {
		return err
	}
	if err := validateNickname(def.Nickname); err != nil {
		return err
	}
	if err := validateIconURL(def.IconURL); err != nil {
		return err
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
	var previous *Definition
	if data, err := os.ReadFile(filepath.Clean(s.definitionPath(name))); err == nil {
		var prior Definition
		if err := parseDefinitionYAML(data, &prior); err != nil {
			return err
		}
		prior.Name = name
		previous = &prior
	} else if !errors.Is(err, os.ErrNotExist) {
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
	if err := fileutil.WriteFileAtomic(s.definitionPath(name), []byte(spec), definitionFilePerm); err != nil {
		return err
	}
	if previous != nil && shouldResetRuntimeForSpecChange(previous, &def) {
		if err := s.resetRuntimeForSpecChange(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func shouldResetRuntimeForSpecChange(previous, next *Definition) bool {
	if previous == nil || next == nil {
		return false
	}
	return !reflect.DeepEqual(previous.Agent, next.Agent)
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
	return s.removeMemoryFile(ctx, name)
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
	rollbackMemory := false
	reassignedSession := false
	defer func() {
		if rollbackNewSpec {
			_ = os.Remove(filepath.Clean(s.definitionPath(newName)))
			if rollbackMemory {
				_ = s.moveMemoryFile(ctx, newName, name)
			}
		}
	}()

	if state != nil && state.SessionID != "" {
		if err := s.cancelAutomataSession(ctx, name, state.SessionID); err != nil {
			return err
		}
		if err := s.reassignSessionUser(ctx, state.SessionID, newName); err != nil {
			return err
		}
		reassignedSession = true
	}

	if err := s.moveMemoryFile(ctx, name, newName); err != nil {
		if reassignedSession {
			_ = s.reassignSessionUser(ctx, state.SessionID, name)
		}
		return err
	}
	rollbackMemory = true

	oldStateDir := filepath.Join(s.stateDir, name)
	newStateDir := filepath.Join(s.stateDir, newName)
	movedState := false
	if _, err := os.Stat(oldStateDir); err == nil {
		if err := os.Rename(oldStateDir, newStateDir); err != nil {
			if reassignedSession {
				_ = s.reassignSessionUser(ctx, state.SessionID, name)
			}
			return err
		}
		movedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		if reassignedSession {
			_ = s.reassignSessionUser(ctx, state.SessionID, name)
		}
		return err
	}

	if err := os.Remove(filepath.Clean(s.definitionPath(name))); err != nil {
		if movedState {
			_ = os.Rename(newStateDir, oldStateDir)
		}
		if reassignedSession {
			_ = s.reassignSessionUser(ctx, state.SessionID, name)
		}
		return err
	}

	rollbackMemory = false
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
	state, err := s.loadState(ctx, name)
	if err != nil {
		return err
	}
	if err := s.assertAutomataTargetAvailable(newName); err != nil {
		return err
	}
	if err := s.PutSpec(ctx, newName, spec); err != nil {
		return err
	}
	if err := s.copyMemoryFile(ctx, name, newName); err != nil {
		_ = os.Remove(filepath.Clean(s.definitionPath(newName)))
		_ = os.RemoveAll(filepath.Join(s.stateDir, newName))
		_ = s.removeMemoryFile(ctx, newName)
		return err
	}
	if state != nil {
		if len(state.TaskTemplates) == 0 && len(state.Tasks) > 0 {
			state.TaskTemplates = cloneTaskTemplatesFromTasks(state.Tasks)
		}
		newState := newInitialState()
		newState.TaskTemplates = append([]TaskTemplate(nil), state.TaskTemplates...)
		if err := s.saveState(ctx, newName, newState); err != nil {
			_ = os.Remove(filepath.Clean(s.definitionPath(newName)))
			_ = os.RemoveAll(filepath.Join(s.stateDir, newName))
			_ = s.removeMemoryFile(ctx, newName)
			return err
		}
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
			Name:          def.Name,
			Kind:          def.Kind,
			Nickname:      def.Nickname,
			IconURL:       def.IconURL,
			Description:   def.Description,
			Purpose:       def.Purpose,
			Goal:          def.Goal,
			Tags:          append([]string(nil), def.Tags...),
			Instruction:   state.Instruction,
			State:         state.State,
			DisplayStatus: view.DisplayStatus,
			Busy:          view.Busy,
			NeedsInput:    view.NeedsInput,
			Disabled:      def.Disabled,
			CurrentRun:    currentRun,
			OpenTaskCount: countTasksByState(state.Tasks, TaskStateOpen),
			DoneTaskCount: countTasksByState(state.Tasks, TaskStateDone),
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
		Definition:    def,
		State:         state,
		AllowedDAGs:   allowed,
		TaskTemplates: append([]TaskTemplate(nil), state.TaskTemplates...),
		CurrentRun:    currentRun,
		RecentRuns:    recentRuns,
		Messages:      messages,
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

func validateNickname(value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("nickname must be a single line")
	}
	if len(value) > 80 {
		return errors.New("nickname must be 80 characters or fewer")
	}
	return nil
}

func validateIconURL(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 2048 {
		return errors.New("icon_url must be 2048 characters or fewer")
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return errors.New("icon_url must be an absolute http(s) URL or root-relative path")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("icon_url must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("icon_url must include a host")
	}
	return nil
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
