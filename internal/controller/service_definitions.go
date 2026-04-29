// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

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
	"gopkg.in/yaml.v3"
)

func (s *Service) definitionPath(name string) string {
	return filepath.Join(s.definitionsDir, name+".yaml")
}

func (s *Service) definitionDirs() []string {
	return []string{s.definitionsDir}
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
	def.ClonedFrom = strings.TrimSpace(def.ClonedFrom)
	def.Nickname = strings.TrimSpace(def.Nickname)
	def.IconURL = strings.TrimSpace(def.IconURL)
	def.Trigger.Prompt = strings.TrimSpace(def.Trigger.Prompt)
	if err := def.Trigger.Validate(); err != nil {
		return err
	}
	if err := validateControllerKind(def.Kind); err != nil {
		return err
	}
	if err := normalizeLabels(&def.Labels); err != nil {
		return err
	}
	if err := validateName(def.Name); err != nil {
		return err
	}
	if def.ClonedFrom != "" {
		if err := validateName(def.ClonedFrom); err != nil {
			return fmt.Errorf("invalid cloned_from: %w", err)
		}
	}
	if err := validateNickname(def.Nickname); err != nil {
		return err
	}
	if err := validateIconURL(def.IconURL); err != nil {
		return err
	}
	if err := s.normalizeWorkflows(ctx, &def.Workflows); err != nil {
		return err
	}
	return nil
}

func (s *Service) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	if err := os.MkdirAll(s.definitionsDir, dirPerm); err != nil {
		return nil, fmt.Errorf("create definitions dir: %w", err)
	}
	defs := make([]*Definition, 0)
	seen := make(map[string]struct{})
	for _, dir := range s.definitionDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read definitions dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if _, ok := seen[name]; ok {
				continue
			}
			def, err := s.loadDefinitionFile(ctx, filepath.Join(dir, entry.Name()))
			if err != nil {
				s.logger.Warn("skipping invalid controller definition", "file", entry.Name(), "error", err)
				continue
			}
			seen[name] = struct{}{}
			defs = append(defs, def)
		}
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

func (s *Service) rememberWorkflow(ctx context.Context, def *Definition, workflowName string) error {
	if def == nil {
		return errors.New("definition is required")
	}
	workflowName = strings.TrimSpace(workflowName)
	if workflowName == "" {
		return errors.New("workflow name is required")
	}
	managed, err := s.workflowIsManaged(ctx, def.Workflows, workflowName)
	if err != nil {
		return err
	}
	if managed {
		return nil
	}

	spec, err := s.GetSpec(ctx, def.Name)
	if err != nil {
		return err
	}
	updatedSpec, err := upsertWorkflowNameInSpec(spec, workflowName)
	if err != nil {
		return err
	}
	if err := s.PutSpec(ctx, def.Name, updatedSpec); err != nil {
		return err
	}

	def.Workflows.Names = append(def.Workflows.Names, workflowName)
	return nil
}

func (s *Service) workflowIsManaged(ctx context.Context, workflows Workflows, workflowName string) (bool, error) {
	for _, name := range workflows.Names {
		if strings.TrimSpace(name) == workflowName {
			return true, nil
		}
	}
	if len(workflows.Labels) == 0 {
		return false, nil
	}
	items, err := s.resolveManagedWorkflowSet(ctx, workflows)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Name == workflowName {
			return true, nil
		}
	}
	return false, nil
}

func upsertWorkflowNameInSpec(spec, workflowName string) (string, error) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(spec), &doc); err != nil {
		return "", fmt.Errorf("parse yaml: %w", err)
	}
	if len(doc) == 0 {
		return "", errors.New("definition is required")
	}

	workflows := asStringMap(doc["workflows"])
	names := appendUniqueString(stringListFromAny(workflows["names"]), workflowName)
	workflows["names"] = names
	doc["workflows"] = workflows

	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	return string(data), nil
}

func asStringMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringListFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	out := make([]string, 0, len(items)+1)
	seen := make(map[string]struct{}, len(items)+1)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if _, ok := seen[value]; !ok {
		out = append(out, value)
	}
	return out
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
		return errors.New("new controller name must be different")
	}
	if err := s.assertControllerTargetAvailable(newName); err != nil {
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
		if err := s.cancelControllerSession(ctx, name, state.SessionID); err != nil {
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

	oldStateDir := filepath.Dir(s.statePath(name))
	newStateDir := filepath.Join(s.stateDir, newName)
	movedState := false
	if _, err := os.Stat(oldStateDir); err == nil {
		if err := os.MkdirAll(filepath.Dir(newStateDir), dirPerm); err != nil {
			if reassignedSession {
				_ = s.reassignSessionUser(ctx, state.SessionID, name)
			}
			return err
		}
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
		return errors.New("duplicate controller name must be different")
	}
	spec, err := s.GetSpec(ctx, name)
	if err != nil {
		return err
	}
	spec, err = annotateClonedFromInSpec(spec, name)
	if err != nil {
		return err
	}
	state, err := s.loadState(ctx, name)
	if err != nil {
		return err
	}
	if err := s.assertControllerTargetAvailable(newName); err != nil {
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
			ClonedFrom:    def.ClonedFrom,
			ResetOnFinish: def.ResetOnFinish,
			Labels:        append([]string(nil), def.Labels...),
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
	workflows, err := s.resolveManagedWorkflowSet(ctx, def.Workflows)
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
		Workflows:     workflows,
		TaskTemplates: append([]TaskTemplate(nil), state.TaskTemplates...),
		CurrentRun:    currentRun,
		RecentRuns:    recentRuns,
		Messages:      messages,
	}, nil
}

func (s *Service) resolveManagedWorkflowSet(ctx context.Context, workflows Workflows) ([]WorkflowInfo, error) {
	seen := make(map[string]WorkflowInfo)
	for _, name := range workflows.Names {
		dag, err := s.dagStore.GetMetadata(ctx, name)
		if err != nil {
			return nil, err
		}
		seen[dag.Name] = WorkflowInfo{
			Name:        dag.Name,
			Description: dag.Description,
			Labels:      dag.Labels.Strings(),
		}
	}
	if len(workflows.Labels) > 0 {
		pg := exec.NewPaginator(1, math.MaxInt)
		result, _, err := s.dagStore.List(ctx, exec.ListDAGsOptions{
			Paginator: &pg,
			Labels:    workflows.Labels,
		})
		if err != nil {
			return nil, err
		}
		for _, dag := range result.Items {
			seen[dag.Name] = WorkflowInfo{
				Name:        dag.Name,
				Description: dag.Description,
				Labels:      dag.Labels.Strings(),
			}
		}
	}
	out := make([]WorkflowInfo, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) listAvailableWorkflowSet(ctx context.Context, workflows Workflows) ([]WorkflowInfo, error) {
	if hasWorkflows(workflows) {
		return s.resolveManagedWorkflowSet(ctx, workflows)
	}

	pg := exec.NewPaginator(1, math.MaxInt)
	result, _, err := s.dagStore.List(ctx, exec.ListDAGsOptions{
		Paginator: &pg,
	})
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowInfo, 0, len(result.Items))
	for _, dag := range result.Items {
		out = append(out, WorkflowInfo{
			Name:        dag.Name,
			Description: dag.Description,
			Labels:      dag.Labels.Strings(),
		})
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
		exec.WithTags([]string{"controller=" + strings.ToLower(name)}),
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

func (s *Service) normalizeWorkflows(ctx context.Context, workflows *Workflows) error {
	if workflows == nil {
		return nil
	}
	for i, name := range workflows.Names {
		workflows.Names[i] = strings.TrimSpace(name)
		if workflows.Names[i] == "" {
			return errors.New("workflows.names contains an empty entry")
		}
		if _, err := s.dagStore.GetMetadata(ctx, workflows.Names[i]); err != nil {
			return fmt.Errorf("workflow %q not found: %w", workflows.Names[i], err)
		}
	}
	for i, label := range workflows.Labels {
		workflows.Labels[i] = strings.TrimSpace(label)
		if workflows.Labels[i] == "" {
			return errors.New("workflows.labels contains an empty entry")
		}
	}
	return nil
}

func hasWorkflows(workflows Workflows) bool {
	return len(workflows.Names) > 0 || len(workflows.Labels) > 0
}

func normalizeLabels(labels *[]string) error {
	if labels == nil {
		return nil
	}
	parsed := core.NewLabels(*labels)
	if err := core.ValidateLabels(parsed); err != nil {
		return fmt.Errorf("invalid labels: %w", err)
	}
	*labels = parsed.Strings()
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
