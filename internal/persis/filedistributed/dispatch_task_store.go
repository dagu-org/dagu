// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

const dispatchTaskStoreVersion = 1

type DispatchTaskStore struct {
	baseDir string
	mu      sync.Mutex
}

type dispatchTaskFile struct {
	Version      int                      `json:"version"`
	Task         *coordinatorv1.Task      `json:"task"`
	TaskFileName string                   `json:"taskFileName"`
	EnqueuedAt   int64                    `json:"enqueuedAt"`
	ClaimToken   string                   `json:"claimToken,omitempty"`
	ClaimedAt    int64                    `json:"claimedAt,omitempty"`
	WorkerID     string                   `json:"workerId,omitempty"`
	PollerID     string                   `json:"pollerId,omitempty"`
	Owner        exec.CoordinatorEndpoint `json:"owner,omitzero"`
}

func NewDispatchTaskStore(baseDir string) *DispatchTaskStore {
	return &DispatchTaskStore{baseDir: baseDir}
}

func (s *DispatchTaskStore) pendingDir() string {
	return filepath.Join(s.baseDir, "pending")
}

func (s *DispatchTaskStore) claimsDir() string {
	return filepath.Join(s.baseDir, "claims")
}

func (s *DispatchTaskStore) claimPath(claimToken string) string {
	return filepath.Join(s.claimsDir(), "claim_"+encodeKey(claimToken)+".json")
}

func (s *DispatchTaskStore) Enqueue(_ context.Context, task *coordinatorv1.Task) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}

	enqueuedAt := time.Now().UTC()
	fileName := fmt.Sprintf("task_%020d_%s.json", enqueuedAt.UnixMilli(), uuid.NewString())
	record := dispatchTaskFile{
		Version:      dispatchTaskStoreVersion,
		Task:         cloneTask(task),
		TaskFileName: fileName,
		EnqueuedAt:   enqueuedAt.UnixMilli(),
	}

	return writeJSONAtomic(filepath.Join(s.pendingDir(), fileName), record)
}

func (s *DispatchTaskStore) ClaimNext(_ context.Context, claim exec.DispatchTaskClaim) (*exec.ClaimedDispatchTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recycleExpiredClaims(claim.ClaimTimeout); err != nil {
		return nil, err
	}

	files, err := sortedFiles(s.pendingDir())
	if err != nil {
		return nil, err
	}

	for _, pendingPath := range files {
		record, err := s.readTaskFile(pendingPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if record.Task == nil || !matchesSelector(claim.Labels, record.Task.WorkerSelector) {
			continue
		}

		claimToken := uuid.NewString()
		claimedAt := time.Now().UTC()
		claimPath := s.claimPath(claimToken)
		if err := ensureDir(filepath.Dir(claimPath)); err != nil {
			return nil, err
		}
		if err := os.Rename(pendingPath, claimPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("claim dispatch task %s: %w", pendingPath, err)
		}
		if err := os.Chtimes(claimPath, claimedAt, claimedAt); err != nil && !os.IsNotExist(err) {
			_ = os.Rename(claimPath, pendingPath)
			return nil, fmt.Errorf("stamp claim file %s: %w", claimPath, err)
		}

		record.ClaimToken = claimToken
		record.ClaimedAt = claimedAt.UnixMilli()
		record.WorkerID = claim.WorkerID
		record.PollerID = claim.PollerID
		record.Owner = claim.Owner
		record.Task, err = applyTaskClaim(record.Task, claim.Owner, claimToken)
		if err != nil {
			_ = os.Rename(claimPath, pendingPath)
			return nil, err
		}

		if err := writeJSONAtomic(claimPath, record); err != nil {
			_ = os.Rename(claimPath, pendingPath)
			return nil, err
		}

		return &exec.ClaimedDispatchTask{
			Task:       cloneTask(record.Task),
			ClaimToken: claimToken,
			ClaimedAt:  claimedAt,
			WorkerID:   claim.WorkerID,
			PollerID:   claim.PollerID,
			Owner:      claim.Owner,
		}, nil
	}

	return nil, nil
}

func (s *DispatchTaskStore) GetClaim(_ context.Context, claimToken string) (*exec.ClaimedDispatchTask, error) {
	record, err := s.readTaskFile(s.claimPath(claimToken))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, exec.ErrDispatchTaskNotFound
		}
		return nil, err
	}
	if record.Task == nil || record.ClaimToken == "" || record.ClaimToken != claimToken || record.ClaimedAt == 0 {
		return nil, exec.ErrDispatchTaskNotFound
	}

	return &exec.ClaimedDispatchTask{
		Task:       cloneTask(record.Task),
		ClaimToken: record.ClaimToken,
		ClaimedAt:  time.UnixMilli(record.ClaimedAt).UTC(),
		WorkerID:   record.WorkerID,
		PollerID:   record.PollerID,
		Owner:      record.Owner,
	}, nil
}

func (s *DispatchTaskStore) DeleteClaim(_ context.Context, claimToken string) error {
	err := os.Remove(s.claimPath(claimToken))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *DispatchTaskStore) CountOutstandingByQueue(_ context.Context, queueName string, claimTimeout time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recycleExpiredClaims(claimTimeout); err != nil {
		return 0, err
	}

	count := 0
	if err := s.scanOutstandingLocked(func(record *dispatchTaskFile) (bool, error) {
		if record == nil || record.Task == nil {
			return false, nil
		}
		if queueName != "" && record.Task.QueueName != queueName {
			return false, nil
		}
		count++
		return false, nil
	}); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *DispatchTaskStore) HasOutstandingAttempt(_ context.Context, attemptKey string, claimTimeout time.Duration) (bool, error) {
	if attemptKey == "" {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recycleExpiredClaims(claimTimeout); err != nil {
		return false, err
	}

	found := false
	if err := s.scanOutstandingLocked(func(record *dispatchTaskFile) (bool, error) {
		if record == nil || record.Task == nil {
			return false, nil
		}
		if record.Task.AttemptKey == attemptKey {
			found = true
			return true, nil
		}
		return false, nil
	}); err != nil {
		return false, err
	}

	return found, nil
}

func (s *DispatchTaskStore) recycleExpiredClaims(claimTimeout time.Duration) error {
	if claimTimeout <= 0 {
		claimTimeout = 30 * time.Second
	}

	files, err := sortedFiles(s.claimsDir())
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, claimPath := range files {
		record, readErr := s.readTaskFile(claimPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return readErr
		}

		var claimedAt time.Time
		if record.ClaimedAt > 0 {
			claimedAt = time.UnixMilli(record.ClaimedAt).UTC()
		}
		if claimedAt.IsZero() {
			info, statErr := os.Stat(claimPath)
			if statErr != nil {
				if os.IsNotExist(statErr) {
					continue
				}
				return statErr
			}
			claimedAt = info.ModTime().UTC()
		}
		if now.Sub(claimedAt) < claimTimeout {
			continue
		}

		record.ClaimToken = ""
		record.ClaimedAt = 0
		record.WorkerID = ""
		record.PollerID = ""
		record.Owner = exec.CoordinatorEndpoint{}
		record.Task = clearTaskClaim(record.Task)
		if err := writeJSONAtomic(claimPath, record); err != nil {
			return err
		}

		pendingPath := filepath.Join(s.pendingDir(), record.TaskFileName)
		if err := ensureDir(filepath.Dir(pendingPath)); err != nil {
			return err
		}
		if err := os.Rename(claimPath, pendingPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("requeue expired claim %s: %w", claimPath, err)
		}
	}

	return nil
}

func (s *DispatchTaskStore) scanOutstandingLocked(match func(*dispatchTaskFile) (bool, error)) error {
	for _, dir := range []string{s.pendingDir(), s.claimsDir()} {
		files, err := sortedFiles(dir)
		if err != nil {
			return err
		}
		for _, path := range files {
			record, readErr := s.readTaskFile(path)
			if readErr != nil {
				if os.IsNotExist(readErr) {
					continue
				}
				return readErr
			}
			stop, err := match(record)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
	}
	return nil
}

func (s *DispatchTaskStore) readTaskFile(path string) (*dispatchTaskFile, error) {
	var record dispatchTaskFile
	if err := readJSONFile(path, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func cloneTask(task *coordinatorv1.Task) *coordinatorv1.Task {
	if task == nil {
		return nil
	}
	cloned, ok := proto.Clone(task).(*coordinatorv1.Task)
	if !ok {
		return nil
	}
	return cloned
}

func applyTaskClaim(task *coordinatorv1.Task, owner exec.CoordinatorEndpoint, claimToken string) (*coordinatorv1.Task, error) {
	task = cloneTask(task)
	if task == nil {
		return nil, nil
	}
	if owner.Port < 0 || owner.Port > math.MaxInt32 {
		return nil, fmt.Errorf("owner coordinator port out of range: %d", owner.Port)
	}
	task.OwnerCoordinatorId = owner.ID
	task.OwnerCoordinatorHost = owner.Host
	task.OwnerCoordinatorPort = int32(owner.Port)
	task.ClaimToken = claimToken
	return task, nil
}

func clearTaskClaim(task *coordinatorv1.Task) *coordinatorv1.Task {
	task = cloneTask(task)
	if task == nil {
		return nil
	}
	task.OwnerCoordinatorId = ""
	task.OwnerCoordinatorHost = ""
	task.OwnerCoordinatorPort = 0
	task.ClaimToken = ""
	task.WorkerId = ""
	return task
}

func matchesSelector(workerLabels, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}
	for key, value := range selector {
		if workerLabels[key] != value {
			return false
		}
	}
	return true
}
