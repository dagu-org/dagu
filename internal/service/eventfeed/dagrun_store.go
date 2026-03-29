// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

var automaticStatuses = []core.Status{
	core.Waiting,
	core.Failed,
	core.Aborted,
}

type dagRunStore struct {
	next     exec.DAGRunStore
	recorder Recorder
}

type attemptContext struct {
	rootName  string
	rootRunID string
	subRunID  string
}

type wrappedAttempt struct {
	next     exec.DAGRunAttempt
	recorder Recorder
	info     attemptContext
}

var _ exec.DAGRunStore = (*dagRunStore)(nil)
var _ exec.DAGRunAttempt = (*wrappedAttempt)(nil)

// WrapDAGRunStore decorates a DAGRunStore so meaningful lifecycle transitions
// are mirrored into the internal event feed after the status write succeeds.
func WrapDAGRunStore(next exec.DAGRunStore, recorder Recorder) exec.DAGRunStore {
	if next == nil || recorder == nil {
		return next
	}
	return &dagRunStore{next: next, recorder: recorder}
}

func (s *dagRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	attempt, err := s.next.CreateAttempt(ctx, dag, ts, dagRunID, opts)
	if err != nil {
		return nil, err
	}
	info := attemptContext{}
	if opts.RootDAGRun != nil {
		info.rootName = opts.RootDAGRun.Name
		info.rootRunID = opts.RootDAGRun.ID
		info.subRunID = dagRunID
	} else if dag != nil {
		info.rootName = dag.Name
		info.rootRunID = dagRunID
	}
	return s.wrapAttempt(attempt, info), nil
}

func (s *dagRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []exec.DAGRunAttempt {
	attempts := s.next.RecentAttempts(ctx, name, itemLimit)
	if len(attempts) == 0 {
		return attempts
	}

	wrapped := make([]exec.DAGRunAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		wrapped = append(wrapped, s.wrapAttempt(attempt, attemptContext{rootName: name}))
	}
	return wrapped
}

func (s *dagRunStore) LatestAttempt(ctx context.Context, name string) (exec.DAGRunAttempt, error) {
	attempt, err := s.next.LatestAttempt(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.wrapAttempt(attempt, attemptContext{rootName: name}), nil
}

func (s *dagRunStore) ListStatuses(ctx context.Context, opts ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return s.next.ListStatuses(ctx, opts...)
}

func (s *dagRunStore) CompareAndSwapLatestAttemptStatus(
	ctx context.Context,
	dagRun exec.DAGRunRef,
	expectedAttemptID string,
	expectedStatus core.Status,
	mutate func(*exec.DAGRunStatus) error,
) (*exec.DAGRunStatus, bool, error) {
	var previousStatus core.Status
	status, swapped, err := s.next.CompareAndSwapLatestAttemptStatus(ctx, dagRun, expectedAttemptID, expectedStatus, func(current *exec.DAGRunStatus) error {
		if current != nil {
			previousStatus = current.Status
		}
		return mutate(current)
	})
	if err != nil || !swapped || status == nil {
		return status, swapped, err
	}

	entry := automaticTransitionEntry(
		attemptContext{rootName: dagRun.Name, rootRunID: dagRun.ID},
		&exec.DAGRunStatus{Status: previousStatus},
		status,
		status.AttemptID,
	)
	if entry != nil {
		s.emit(ctx, *entry)
	}
	return status, swapped, nil
}

func (s *dagRunStore) FindAttempt(ctx context.Context, dagRun exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	attempt, err := s.next.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, err
	}
	return s.wrapAttempt(attempt, attemptContext{rootName: dagRun.Name, rootRunID: dagRun.ID}), nil
}

func (s *dagRunStore) FindSubAttempt(ctx context.Context, dagRun exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	attempt, err := s.next.FindSubAttempt(ctx, dagRun, subDAGRunID)
	if err != nil {
		return nil, err
	}
	return s.wrapAttempt(attempt, attemptContext{
		rootName:  dagRun.Name,
		rootRunID: dagRun.ID,
		subRunID:  subDAGRunID,
	}), nil
}

func (s *dagRunStore) CreateSubAttempt(ctx context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	attempt, err := s.next.CreateSubAttempt(ctx, rootRef, subDAGRunID)
	if err != nil {
		return nil, err
	}
	return s.wrapAttempt(attempt, attemptContext{
		rootName:  rootRef.Name,
		rootRunID: rootRef.ID,
		subRunID:  subDAGRunID,
	}), nil
}

func (s *dagRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int, opts ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return s.next.RemoveOldDAGRuns(ctx, name, retentionDays, opts...)
}

func (s *dagRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	return s.next.RenameDAGRuns(ctx, oldName, newName)
}

func (s *dagRunStore) RemoveDAGRun(ctx context.Context, dagRun exec.DAGRunRef) error {
	return s.next.RemoveDAGRun(ctx, dagRun)
}

func (s *dagRunStore) wrapAttempt(attempt exec.DAGRunAttempt, info attemptContext) exec.DAGRunAttempt {
	if attempt == nil {
		return nil
	}
	return &wrappedAttempt{
		next:     attempt,
		recorder: s.recorder,
		info:     info,
	}
}

func (s *dagRunStore) emit(ctx context.Context, entry Entry) {
	if err := s.recorder.Record(ctx, entry); err != nil {
		logger.Warn(ctx, "Failed to record recent event",
			slog.String("event_type", string(entry.Type)),
			tag.DAG(entry.DAGName),
			tag.RunID(entry.DAGRunID),
			tag.Error(err),
		)
	}
}

func (a *wrappedAttempt) ID() string {
	return a.next.ID()
}

func (a *wrappedAttempt) Open(ctx context.Context) error {
	return a.next.Open(ctx)
}

func (a *wrappedAttempt) Write(ctx context.Context, status exec.DAGRunStatus) error {
	var previous *exec.DAGRunStatus
	if current, err := a.next.ReadStatus(ctx); err == nil {
		previous = current
	}

	if err := a.next.Write(ctx, status); err != nil {
		return err
	}

	entry := automaticTransitionEntry(a.info, previous, &status, a.next.ID())
	if entry != nil {
		a.emit(ctx, *entry)
	}

	return nil
}

func (a *wrappedAttempt) Close(ctx context.Context) error {
	return a.next.Close(ctx)
}

func (a *wrappedAttempt) ReadStatus(ctx context.Context) (*exec.DAGRunStatus, error) {
	return a.next.ReadStatus(ctx)
}

func (a *wrappedAttempt) ReadDAG(ctx context.Context) (*core.DAG, error) {
	return a.next.ReadDAG(ctx)
}

func (a *wrappedAttempt) SetDAG(dag *core.DAG) {
	a.next.SetDAG(dag)
}

func (a *wrappedAttempt) Abort(ctx context.Context) error {
	return a.next.Abort(ctx)
}

func (a *wrappedAttempt) IsAborting(ctx context.Context) (bool, error) {
	return a.next.IsAborting(ctx)
}

func (a *wrappedAttempt) Hide(ctx context.Context) error {
	return a.next.Hide(ctx)
}

func (a *wrappedAttempt) Hidden() bool {
	return a.next.Hidden()
}

func (a *wrappedAttempt) WriteOutputs(ctx context.Context, outputs *exec.DAGRunOutputs) error {
	return a.next.WriteOutputs(ctx, outputs)
}

func (a *wrappedAttempt) ReadOutputs(ctx context.Context) (*exec.DAGRunOutputs, error) {
	return a.next.ReadOutputs(ctx)
}

func (a *wrappedAttempt) WriteStepMessages(ctx context.Context, stepName string, messages []exec.LLMMessage) error {
	return a.next.WriteStepMessages(ctx, stepName, messages)
}

func (a *wrappedAttempt) ReadStepMessages(ctx context.Context, stepName string) ([]exec.LLMMessage, error) {
	return a.next.ReadStepMessages(ctx, stepName)
}

func (a *wrappedAttempt) WorkDir() string {
	return a.next.WorkDir()
}

func (a *wrappedAttempt) emit(ctx context.Context, entry Entry) {
	if err := a.recorder.Record(ctx, entry); err != nil {
		logger.Warn(ctx, "Failed to record recent event",
			slog.String("event_type", string(entry.Type)),
			tag.DAG(entry.DAGName),
			tag.RunID(entry.DAGRunID),
			tag.Error(err),
		)
	}
}

func automaticTransitionEntry(
	info attemptContext,
	previous *exec.DAGRunStatus,
	current *exec.DAGRunStatus,
	fallbackAttemptID string,
) *Entry {
	if current == nil || !slices.Contains(automaticStatuses, current.Status) {
		return nil
	}
	if previous != nil && previous.Status == current.Status {
		return nil
	}

	prevMeaningful, ok := previousMeaningfulStatus(previous)
	if ok && prevMeaningful == current.Status {
		return nil
	}

	entry := &Entry{
		Type:               eventTypeForStatus(current.Status),
		Timestamp:          transitionTimestamp(current),
		DAGName:            resolveRootName(info, current),
		DAGRunID:           resolveRootRunID(info, current),
		AttemptID:          resolveAttemptID(current, fallbackAttemptID),
		Reason:             current.Error,
		ResultingRunStatus: current.Status.String(),
	}
	if subRunID := resolveSubRunID(info, current); subRunID != "" {
		entry.SubDAGRunID = subRunID
	}

	switch current.Status {
	case core.Waiting:
		if node := firstWaitingNode(current); node != nil {
			if ts := latestNodeTime(node); !ts.IsZero() {
				entry.Timestamp = ts.UTC()
			}
			entry.StepName = node.Step.Name
			entry.ApprovalIteration = new(node.ApprovalIteration)
		}
	case core.Failed:
		entry.StepName = representativeStepName(current, core.NodeFailed)
	case core.Aborted:
		entry.StepName = representativeStepName(current, core.NodeAborted)
	case core.NotStarted, core.Running, core.Succeeded, core.Queued, core.PartiallySucceeded, core.Rejected:
	}

	return entry
}

func previousMeaningfulStatus(status *exec.DAGRunStatus) (core.Status, bool) {
	if status == nil {
		return core.NotStarted, false
	}
	if slices.Contains(automaticStatuses, status.Status) {
		return status.Status, true
	}
	return core.NotStarted, false
}

func eventTypeForStatus(status core.Status) EventType {
	switch status {
	case core.Waiting:
		return EventTypeWaiting
	case core.Failed:
		return EventTypeFailed
	case core.Aborted:
		return EventTypeAborted
	case core.NotStarted, core.Running, core.Succeeded, core.Queued, core.PartiallySucceeded, core.Rejected:
		return EventTypeFailed
	}

	return EventTypeFailed
}

func resolveRootName(info attemptContext, status *exec.DAGRunStatus) string {
	if info.rootName != "" {
		return info.rootName
	}
	if status != nil && status.Root.Name != "" {
		return status.Root.Name
	}
	if status != nil {
		return status.Name
	}
	return ""
}

func resolveRootRunID(info attemptContext, status *exec.DAGRunStatus) string {
	if info.rootRunID != "" {
		return info.rootRunID
	}
	if status != nil && status.Root.ID != "" && status.Root.ID != status.DAGRunID {
		return status.Root.ID
	}
	if status != nil {
		return status.DAGRunID
	}
	return ""
}

func resolveSubRunID(info attemptContext, status *exec.DAGRunStatus) string {
	if info.subRunID != "" {
		return info.subRunID
	}
	if status != nil && status.Root.ID != "" && status.Root.ID != status.DAGRunID {
		return status.DAGRunID
	}
	return ""
}

func resolveAttemptID(status *exec.DAGRunStatus, fallbackAttemptID string) string {
	if status != nil && status.AttemptID != "" {
		return status.AttemptID
	}
	return fallbackAttemptID
}

func firstWaitingNode(status *exec.DAGRunStatus) *exec.Node {
	if status == nil {
		return nil
	}
	for _, node := range status.Nodes {
		if node != nil && node.Status == core.NodeWaiting {
			return node
		}
	}
	return nil
}

func representativeStepName(status *exec.DAGRunStatus, wanted core.NodeStatus) string {
	if status == nil {
		return ""
	}
	var (
		name     string
		bestTime time.Time
	)
	for _, node := range status.Nodes {
		if node == nil || node.Status != wanted || node.Step.Name == "" {
			continue
		}
		nodeTime := latestNodeTime(node)
		if name == "" || nodeTime.After(bestTime) {
			name = node.Step.Name
			bestTime = nodeTime
		}
	}
	return name
}

func latestNodeTime(node *exec.Node) time.Time {
	if node == nil {
		return time.Time{}
	}
	if t, err := stringutil.ParseTime(node.FinishedAt); err == nil && !t.IsZero() {
		return t
	}
	if t, err := stringutil.ParseTime(node.StartedAt); err == nil && !t.IsZero() {
		return t
	}
	return time.Time{}
}

func transitionTimestamp(status *exec.DAGRunStatus) time.Time {
	if status == nil {
		return time.Now().UTC()
	}
	if t, err := stringutil.ParseTime(status.FinishedAt); err == nil && !t.IsZero() {
		return t.UTC()
	}
	if t, err := stringutil.ParseTime(status.StartedAt); err == nil && !t.IsZero() {
		return t.UTC()
	}
	return time.Now().UTC()
}
