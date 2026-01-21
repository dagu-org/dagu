// Copyright (C) 2025 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/gitsync"
)

// SyncService is the interface for Git sync operations.
type SyncService interface {
	Pull(ctx context.Context) (*gitsync.SyncResult, error)
	Publish(ctx context.Context, dagID, message string, force bool) (*gitsync.SyncResult, error)
	PublishAll(ctx context.Context, message string) (*gitsync.SyncResult, error)
	Discard(ctx context.Context, dagID string) error
	GetStatus(ctx context.Context) (*gitsync.OverallStatus, error)
	GetDAGStatus(ctx context.Context, dagID string) (*gitsync.DAGState, error)
	GetConfig(ctx context.Context) (*gitsync.Config, error)
	UpdateConfig(ctx context.Context, cfg *gitsync.Config) error
	TestConnection(ctx context.Context) (*gitsync.ConnectionResult, error)
}

// GetSyncStatus returns the overall Git sync status.
func (a *API) GetSyncStatus(ctx context.Context, _ api.GetSyncStatusRequestObject) (api.GetSyncStatusResponseObject, error) {
	// Check if sync service is available
	if a.syncService == nil {
		// Return disabled status when sync is not configured
		return api.GetSyncStatus200JSONResponse{
			Enabled: false,
			Summary: api.SyncSummarySynced,
			Counts: api.SyncStatusCounts{
				Synced:    0,
				Modified:  0,
				Untracked: 0,
				Conflict:  0,
			},
		}, nil
	}

	status, err := a.syncService.GetStatus(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.GetSyncStatus200JSONResponse{
		Enabled:        status.Enabled,
		Repository:     ptrOf(status.Repository),
		Branch:         ptrOf(status.Branch),
		Summary:        toAPISyncSummary(status.Summary),
		LastSyncAt:     status.LastSyncAt,
		LastSyncCommit: ptrOf(status.LastSyncCommit),
		LastSyncStatus: ptrOf(status.LastSyncStatus),
		LastError:      status.LastError,
		Dags:           toAPISyncDAGStates(status.DAGs),
		Counts:         toAPISyncCounts(status.Counts),
	}, nil
}

// SyncPull pulls changes from the remote repository.
func (a *API) SyncPull(ctx context.Context, _ api.SyncPullRequestObject) (api.SyncPullResponseObject, error) {
	if a.syncService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Git sync is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	result, err := a.syncService.Pull(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.SyncPull200JSONResponse(toAPISyncResult(result)), nil
}

// SyncPublishAll publishes all modified DAGs.
func (a *API) SyncPublishAll(ctx context.Context, req api.SyncPublishAllRequestObject) (api.SyncPublishAllResponseObject, error) {
	if a.syncService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Git sync is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	var message string
	if req.Body != nil && req.Body.Message != nil {
		message = *req.Body.Message
	}

	result, err := a.syncService.PublishAll(ctx, message)
	if err != nil {
		if gitsync.IsNotEnabled(err) {
			return nil, &Error{
				Code:       api.ErrorCodeInternalError,
				Message:    "Git sync is not enabled",
				HTTPStatus: http.StatusServiceUnavailable,
			}
		}
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.SyncPublishAll200JSONResponse(toAPISyncResult(result)), nil
}

// SyncTestConnection tests the connection to the remote repository.
func (a *API) SyncTestConnection(ctx context.Context, _ api.SyncTestConnectionRequestObject) (api.SyncTestConnectionResponseObject, error) {
	if a.syncService == nil {
		return api.SyncTestConnection200JSONResponse{
			Success: false,
			Error:   ptrOf("Git sync is not configured"),
		}, nil
	}

	result, err := a.syncService.TestConnection(ctx)
	if err != nil {
		return api.SyncTestConnection200JSONResponse{
			Success: false,
			Error:   ptrOf(err.Error()),
		}, nil
	}

	return api.SyncTestConnection200JSONResponse{
		Success: result.Success,
		Message: ptrOf(result.Message),
		Error:   ptrOf(result.Error),
	}, nil
}

// GetSyncConfig returns the Git sync configuration.
func (a *API) GetSyncConfig(ctx context.Context, _ api.GetSyncConfigRequestObject) (api.GetSyncConfigResponseObject, error) {
	if a.syncService == nil {
		return api.GetSyncConfig200JSONResponse{
			Enabled: false,
		}, nil
	}

	cfg, err := a.syncService.GetConfig(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.GetSyncConfig200JSONResponse{
		Enabled:     cfg.Enabled,
		Repository:  ptrOf(cfg.Repository),
		Branch:      ptrOf(cfg.Branch),
		Path:        ptrOf(cfg.Path),
		PushEnabled: ptrOf(cfg.PushEnabled),
		Auth: &api.SyncAuthConfig{
			Type:       api.SyncAuthConfigType(cfg.Auth.Type),
			SshKeyPath: ptrOf(cfg.Auth.SSHKeyPath),
			// Token is not returned for security
		},
		AutoSync: &api.SyncAutoSyncConfig{
			Enabled:   cfg.AutoSync.Enabled,
			OnStartup: cfg.AutoSync.OnStartup,
			Interval:  cfg.AutoSync.Interval,
		},
		Commit: &api.SyncCommitConfig{
			AuthorName:  ptrOf(cfg.Commit.AuthorName),
			AuthorEmail: ptrOf(cfg.Commit.AuthorEmail),
		},
	}, nil
}

// UpdateSyncConfig updates the Git sync configuration.
func (a *API) UpdateSyncConfig(ctx context.Context, req api.UpdateSyncConfigRequestObject) (api.UpdateSyncConfigResponseObject, error) {
	if a.syncService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Git sync is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	cfg, err := a.syncService.GetConfig(ctx)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	// Update fields from request
	if req.Body.Enabled != nil {
		cfg.Enabled = *req.Body.Enabled
	}
	if req.Body.Repository != nil {
		cfg.Repository = *req.Body.Repository
	}
	if req.Body.Branch != nil {
		cfg.Branch = *req.Body.Branch
	}
	if req.Body.Path != nil {
		cfg.Path = *req.Body.Path
	}
	if req.Body.PushEnabled != nil {
		cfg.PushEnabled = *req.Body.PushEnabled
	}
	if req.Body.Auth != nil {
		cfg.Auth.Type = string(req.Body.Auth.Type)
		if req.Body.Auth.Token != nil {
			cfg.Auth.Token = *req.Body.Auth.Token
		}
		if req.Body.Auth.SshKeyPath != nil {
			cfg.Auth.SSHKeyPath = *req.Body.Auth.SshKeyPath
		}
	}
	if req.Body.AutoSync != nil {
		cfg.AutoSync.Enabled = req.Body.AutoSync.Enabled
		cfg.AutoSync.OnStartup = req.Body.AutoSync.OnStartup
		cfg.AutoSync.Interval = req.Body.AutoSync.Interval
	}
	if req.Body.Commit != nil {
		if req.Body.Commit.AuthorName != nil {
			cfg.Commit.AuthorName = *req.Body.Commit.AuthorName
		}
		if req.Body.Commit.AuthorEmail != nil {
			cfg.Commit.AuthorEmail = *req.Body.Commit.AuthorEmail
		}
	}

	if err := a.syncService.UpdateConfig(ctx, cfg); err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	// Return the updated config
	return api.UpdateSyncConfig200JSONResponse{
		Enabled:     cfg.Enabled,
		Repository:  ptrOf(cfg.Repository),
		Branch:      ptrOf(cfg.Branch),
		Path:        ptrOf(cfg.Path),
		PushEnabled: ptrOf(cfg.PushEnabled),
		Auth: &api.SyncAuthConfig{
			Type:       api.SyncAuthConfigType(cfg.Auth.Type),
			SshKeyPath: ptrOf(cfg.Auth.SSHKeyPath),
			// Token is not returned for security
		},
		AutoSync: &api.SyncAutoSyncConfig{
			Enabled:   cfg.AutoSync.Enabled,
			OnStartup: cfg.AutoSync.OnStartup,
			Interval:  cfg.AutoSync.Interval,
		},
		Commit: &api.SyncCommitConfig{
			AuthorName:  ptrOf(cfg.Commit.AuthorName),
			AuthorEmail: ptrOf(cfg.Commit.AuthorEmail),
		},
	}, nil
}

// PublishDag publishes a single DAG.
func (a *API) PublishDag(ctx context.Context, req api.PublishDagRequestObject) (api.PublishDagResponseObject, error) {
	if a.syncService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Git sync is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	var message string
	var force bool
	if req.Body != nil {
		if req.Body.Message != nil {
			message = *req.Body.Message
		}
		if req.Body.Force != nil {
			force = *req.Body.Force
		}
	}

	result, err := a.syncService.Publish(ctx, req.Name, message, force)
	if err != nil {
		var conflictErr *gitsync.ConflictError
		if gitsync.IsConflict(err) {
			if conflictErr, _ = err.(*gitsync.ConflictError); conflictErr != nil {
				return api.PublishDag409JSONResponse{
					DagId:         conflictErr.DAGID,
					RemoteCommit:  ptrOf(conflictErr.RemoteCommit),
					RemoteAuthor:  ptrOf(conflictErr.RemoteAuthor),
					RemoteMessage: ptrOf(conflictErr.RemoteMessage),
					Message:       conflictErr.Error(),
				}, nil
			}
			return api.PublishDag409JSONResponse{
				DagId:   req.Name,
				Message: err.Error(),
			}, nil
		}
		if gitsync.IsDAGNotFound(err) {
			return api.PublishDag404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: err.Error(),
			}, nil
		}
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.PublishDag200JSONResponse(toAPISyncResult(result)), nil
}

// DiscardDagChanges discards local changes for a DAG.
func (a *API) DiscardDagChanges(ctx context.Context, req api.DiscardDagChangesRequestObject) (api.DiscardDagChangesResponseObject, error) {
	if a.syncService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Git sync is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	if err := a.syncService.Discard(ctx, req.Name); err != nil {
		if gitsync.IsDAGNotFound(err) {
			return api.DiscardDagChanges404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: err.Error(),
			}, nil
		}
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return api.DiscardDagChanges200JSONResponse{
		Message: "Changes discarded successfully",
	}, nil
}

// Helper functions for type conversion

func toAPISyncSummary(s gitsync.SummaryStatus) api.SyncSummary {
	switch s {
	case gitsync.SummarySynced:
		return api.SyncSummarySynced
	case gitsync.SummaryPending:
		return api.SyncSummaryPending
	case gitsync.SummaryConflict:
		return api.SyncSummaryConflict
	case gitsync.SummaryError:
		return api.SyncSummaryError
	default:
		return api.SyncSummarySynced
	}
}

func toAPISyncStatus(s gitsync.SyncStatus) api.SyncStatus {
	switch s {
	case gitsync.StatusSynced:
		return api.SyncStatusSynced
	case gitsync.StatusModified:
		return api.SyncStatusModified
	case gitsync.StatusUntracked:
		return api.SyncStatusUntracked
	case gitsync.StatusConflict:
		return api.SyncStatusConflict
	default:
		return api.SyncStatusSynced
	}
}

func toAPISyncDAGStates(states map[string]*gitsync.DAGState) *map[string]api.SyncDAGState {
	if states == nil {
		return nil
	}
	result := make(map[string]api.SyncDAGState)
	for id, state := range states {
		result[id] = api.SyncDAGState{
			Status:             toAPISyncStatus(state.Status),
			BaseCommit:         ptrOf(state.BaseCommit),
			LastSyncedHash:     ptrOf(state.LastSyncedHash),
			LastSyncedAt:       state.LastSyncedAt,
			ModifiedAt:         state.ModifiedAt,
			LocalHash:          ptrOf(state.LocalHash),
			RemoteCommit:       ptrOf(state.RemoteCommit),
			RemoteAuthor:       ptrOf(state.RemoteAuthor),
			RemoteMessage:      ptrOf(state.RemoteMessage),
			ConflictDetectedAt: state.ConflictDetectedAt,
		}
	}
	return &result
}

func toAPISyncCounts(counts gitsync.StatusCounts) api.SyncStatusCounts {
	return api.SyncStatusCounts{
		Synced:    counts.Synced,
		Modified:  counts.Modified,
		Untracked: counts.Untracked,
		Conflict:  counts.Conflict,
	}
}

func toAPISyncResult(result *gitsync.SyncResult) api.SyncResultResponse {
	var errors *[]api.SyncError
	if len(result.Errors) > 0 {
		errList := make([]api.SyncError, len(result.Errors))
		for i, e := range result.Errors {
			errList[i] = api.SyncError{
				DagId:   ptrOf(e.DAGID),
				Message: e.Message,
			}
		}
		errors = &errList
	}

	return api.SyncResultResponse{
		Success:   result.Success,
		Message:   ptrOf(result.Message),
		Synced:    ptrOf(result.Synced),
		Modified:  ptrOf(result.Modified),
		Conflicts: ptrOf(result.Conflicts),
		Errors:    errors,
		Timestamp: result.Timestamp,
	}
}

