// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"context"
	"errors"
	"time"

	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var (
	ErrDispatchTaskNotFound = errors.New("dispatch task claim not found")
	ErrDAGRunLeaseNotFound  = errors.New("dag-run lease not found")
)

// CoordinatorEndpoint identifies a coordinator instance that owns a
// distributed task or run.
type CoordinatorEndpoint struct {
	ID   string `json:"id,omitempty"`
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

// HostInfo converts the endpoint to the existing service registry host shape.
func (e CoordinatorEndpoint) HostInfo() HostInfo {
	return HostInfo{
		ID:   e.ID,
		Host: e.Host,
		Port: e.Port,
	}
}

// CoordinatorEndpointFromHostInfo converts a host record to an endpoint.
func CoordinatorEndpointFromHostInfo(info HostInfo) CoordinatorEndpoint {
	return CoordinatorEndpoint{
		ID:   info.ID,
		Host: info.Host,
		Port: info.Port,
	}
}

// DispatchTaskClaim describes a worker poller's attempt to claim a shared
// distributed task.
type DispatchTaskClaim struct {
	WorkerID     string
	PollerID     string
	Labels       map[string]string
	Owner        CoordinatorEndpoint
	ClaimTimeout time.Duration
}

// ClaimedDispatchTask is a shared pending task that has been claimed by a
// specific worker poller and must be acknowledged before execution begins.
type ClaimedDispatchTask struct {
	Task       *coordinatorv1.Task
	ClaimToken string
	ClaimedAt  time.Time
	WorkerID   string
	PollerID   string
	Owner      CoordinatorEndpoint
}

// DispatchTaskStore manages the shared distributed dispatch queue.
type DispatchTaskStore interface {
	Enqueue(ctx context.Context, task *coordinatorv1.Task) error
	ClaimNext(ctx context.Context, claim DispatchTaskClaim) (*ClaimedDispatchTask, error)
	GetClaim(ctx context.Context, claimToken string) (*ClaimedDispatchTask, error)
	DeleteClaim(ctx context.Context, claimToken string) error
	CountOutstandingByQueue(ctx context.Context, queueName string, claimTimeout time.Duration) (int, error)
	HasOutstandingAttempt(ctx context.Context, attemptKey string, claimTimeout time.Duration) (bool, error)
}

// WorkerHeartbeatRecord is the shared presence record for a worker.
type WorkerHeartbeatRecord struct {
	WorkerID        string                     `json:"workerId"`
	Labels          map[string]string          `json:"labels,omitempty"`
	Stats           *coordinatorv1.WorkerStats `json:"stats,omitempty"`
	LastHeartbeatAt int64                      `json:"lastHeartbeatAt"`
}

// LastHeartbeatTime returns the last heartbeat as a time.
func (r WorkerHeartbeatRecord) LastHeartbeatTime() time.Time {
	if r.LastHeartbeatAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(r.LastHeartbeatAt).UTC()
}

// WorkerHeartbeatStore persists shared worker presence across coordinators.
type WorkerHeartbeatStore interface {
	Upsert(ctx context.Context, record WorkerHeartbeatRecord) error
	List(ctx context.Context) ([]WorkerHeartbeatRecord, error)
	DeleteStale(ctx context.Context, before time.Time) (int, error)
}

// DAGRunLease is the shared liveness record for an active distributed attempt.
type DAGRunLease struct {
	AttemptKey      string              `json:"attemptKey"`
	DAGRun          DAGRunRef           `json:"dagRun"`
	Root            DAGRunRef           `json:"root,omitzero"`
	AttemptID       string              `json:"attemptId"`
	QueueName       string              `json:"queueName"`
	WorkerID        string              `json:"workerId"`
	Owner           CoordinatorEndpoint `json:"owner"`
	ClaimedAt       int64               `json:"claimedAt"`
	LastHeartbeatAt int64               `json:"lastHeartbeatAt"`
}

// LastHeartbeatTime returns the last run heartbeat time.
func (l DAGRunLease) LastHeartbeatTime() time.Time {
	if l.LastHeartbeatAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(l.LastHeartbeatAt).UTC()
}

// ClaimedTime returns the lease creation time.
func (l DAGRunLease) ClaimedTime() time.Time {
	if l.ClaimedAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(l.ClaimedAt).UTC()
}

// IsFresh reports whether the lease is still alive.
func (l DAGRunLease) IsFresh(now time.Time, staleThreshold time.Duration) bool {
	if staleThreshold <= 0 || l.LastHeartbeatAt == 0 {
		return false
	}
	return now.Sub(l.LastHeartbeatTime()) < staleThreshold
}

// DAGRunLeaseStore persists active distributed attempt leases.
type DAGRunLeaseStore interface {
	Upsert(ctx context.Context, lease DAGRunLease) error
	Touch(ctx context.Context, attemptKey string, observedAt time.Time) error
	Delete(ctx context.Context, attemptKey string) error
	Get(ctx context.Context, attemptKey string) (*DAGRunLease, error)
	ListByQueue(ctx context.Context, queueName string) ([]DAGRunLease, error)
	ListAll(ctx context.Context) ([]DAGRunLease, error)
}
