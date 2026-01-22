package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/gitsync"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// SyncService is the interface for Git sync operations.
type SyncService interface {
	Pull(ctx context.Context) (*gitsync.SyncResult, error)
	Publish(ctx context.Context, dagID, message string, force bool) (*gitsync.SyncResult, error)
	PublishAll(ctx context.Context, message string) (*gitsync.SyncResult, error)
	Discard(ctx context.Context, dagID string) error
	GetStatus(ctx context.Context) (*gitsync.OverallStatus, error)
	GetDAGStatus(ctx context.Context, dagID string) (*gitsync.DAGState, error)
	GetDAGDiff(ctx context.Context, dagID string) (*gitsync.DAGDiff, error)
	GetConfig(ctx context.Context) (*gitsync.Config, error)
	UpdateConfig(ctx context.Context, cfg *gitsync.Config) error
	TestConnection(ctx context.Context) (*gitsync.ConnectionResult, error)
}

// errSyncNotConfigured returns an error for when sync service is not available.
var errSyncNotConfigured = &Error{
	Code:       api.ErrorCodeInternalError,
	Message:    "Git sync is not configured",
	HTTPStatus: http.StatusServiceUnavailable,
}

// requireSyncService checks if the sync service is available.
func (a *API) requireSyncService() error {
	if a.syncService == nil {
		return errSyncNotConfigured
	}
	return nil
}

// internalError creates an internal server error from an error.
func internalError(err error) *Error {
	return &Error{
		Code:       api.ErrorCodeInternalError,
		Message:    err.Error(),
		HTTPStatus: http.StatusInternalServerError,
	}
}

// GetSyncStatus returns the overall Git sync status.
func (a *API) GetSyncStatus(ctx context.Context, _ api.GetSyncStatusRequestObject) (api.GetSyncStatusResponseObject, error) {
	if a.syncService == nil {
		return disabledSyncStatusResponse(), nil
	}

	status, err := a.syncService.GetStatus(ctx)
	if err != nil {
		return nil, internalError(err)
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

// disabledSyncStatusResponse returns a response for when sync is disabled.
func disabledSyncStatusResponse() api.GetSyncStatus200JSONResponse {
	return api.GetSyncStatus200JSONResponse{
		Enabled: false,
		Summary: api.SyncSummarySynced,
		Counts: api.SyncStatusCounts{
			Synced:    0,
			Modified:  0,
			Untracked: 0,
			Conflict:  0,
		},
	}
}

// SyncPull pulls changes from the remote repository.
func (a *API) SyncPull(ctx context.Context, _ api.SyncPullRequestObject) (api.SyncPullResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	result, err := a.syncService.Pull(ctx)
	if err != nil {
		return nil, internalError(err)
	}

	// Audit log
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]any{
			"synced":    result.Synced,
			"modified":  result.Modified,
			"conflicts": result.Conflicts,
		})
		entry := audit.NewEntry(audit.CategoryGitSync, "sync_pull", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.SyncPull200JSONResponse(toAPISyncResult(result)), nil
}

// SyncPublishAll publishes all modified DAGs.
func (a *API) SyncPublishAll(ctx context.Context, req api.SyncPublishAllRequestObject) (api.SyncPublishAllResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	var message string
	if req.Body != nil && req.Body.Message != nil {
		message = *req.Body.Message
	}

	result, err := a.syncService.PublishAll(ctx, message)
	if err != nil {
		if gitsync.IsNotEnabled(err) {
			return nil, errSyncNotConfigured
		}
		return nil, internalError(err)
	}

	// Audit log
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]any{
			"message":  message,
			"synced":   result.Synced,
			"modified": result.Modified,
		})
		entry := audit.NewEntry(audit.CategoryGitSync, "sync_publish_all", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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
		return api.GetSyncConfig200JSONResponse{Enabled: false}, nil
	}

	cfg, err := a.syncService.GetConfig(ctx)
	if err != nil {
		return nil, internalError(err)
	}

	return api.GetSyncConfig200JSONResponse(toAPISyncConfig(cfg)), nil
}

// GetSyncDAGDiff returns the diff between local and remote versions of a DAG.
func (a *API) GetSyncDAGDiff(ctx context.Context, req api.GetSyncDAGDiffRequestObject) (api.GetSyncDAGDiffResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}

	diff, err := a.syncService.GetDAGDiff(ctx, req.Name)
	if err != nil {
		if gitsync.IsDAGNotFound(err) {
			return api.GetSyncDAGDiff404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: err.Error(),
			}, nil
		}
		return nil, internalError(err)
	}

	return api.GetSyncDAGDiff200JSONResponse{
		DagId:         diff.DAGID,
		Status:        toAPISyncStatus(diff.Status),
		LocalContent:  diff.LocalContent,
		RemoteContent: ptrOf(diff.RemoteContent),
		RemoteCommit:  ptrOf(diff.RemoteCommit),
		RemoteAuthor:  ptrOf(diff.RemoteAuthor),
		RemoteMessage: ptrOf(diff.RemoteMessage),
	}, nil
}

// UpdateSyncConfig updates the Git sync configuration.
func (a *API) UpdateSyncConfig(ctx context.Context, req api.UpdateSyncConfigRequestObject) (api.UpdateSyncConfigResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	cfg, err := a.syncService.GetConfig(ctx)
	if err != nil {
		return nil, internalError(err)
	}

	applyConfigUpdates(cfg, req.Body)

	if err := a.syncService.UpdateConfig(ctx, cfg); err != nil {
		return nil, internalError(err)
	}

	// Audit log (exclude sensitive auth fields)
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]any{
			"enabled":      cfg.Enabled,
			"repository":   cfg.Repository,
			"branch":       cfg.Branch,
			"push_enabled": cfg.PushEnabled,
		})
		entry := audit.NewEntry(audit.CategoryGitSync, "sync_config_update", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.UpdateSyncConfig200JSONResponse(toAPISyncConfig(cfg)), nil
}

// PublishDag publishes a single DAG.
func (a *API) PublishDag(ctx context.Context, req api.PublishDagRequestObject) (api.PublishDagResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	message, force := extractPublishOptions(req.Body)

	result, err := a.syncService.Publish(ctx, req.Name, message, force)
	if err != nil {
		return handlePublishError(err, req.Name)
	}

	// Audit log
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]any{
			"dag_id":  req.Name,
			"message": message,
			"force":   force,
		})
		entry := audit.NewEntry(audit.CategoryGitSync, "sync_publish", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.PublishDag200JSONResponse(toAPISyncResult(result)), nil
}

// DiscardDagChanges discards local changes for a DAG.
func (a *API) DiscardDagChanges(ctx context.Context, req api.DiscardDagChangesRequestObject) (api.DiscardDagChangesResponseObject, error) {
	if err := a.requireSyncService(); err != nil {
		return nil, err
	}
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	if err := a.syncService.Discard(ctx, req.Name); err != nil {
		if gitsync.IsDAGNotFound(err) {
			return api.DiscardDagChanges404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: err.Error(),
			}, nil
		}
		return nil, internalError(err)
	}

	// Audit log
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{
			"dag_id": req.Name,
		})
		entry := audit.NewEntry(audit.CategoryGitSync, "sync_discard", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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

// toAPISyncConfig converts a gitsync.Config to the API response format.
func toAPISyncConfig(cfg *gitsync.Config) api.SyncConfigResponse {
	return api.SyncConfigResponse{
		Enabled:     cfg.Enabled,
		Repository:  ptrOf(cfg.Repository),
		Branch:      ptrOf(cfg.Branch),
		Path:        ptrOf(cfg.Path),
		PushEnabled: ptrOf(cfg.PushEnabled),
		Auth: &api.SyncAuthConfig{
			Type:       api.SyncAuthConfigType(cfg.Auth.Type),
			SshKeyPath: ptrOf(cfg.Auth.SSHKeyPath),
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
	}
}

// applyConfigUpdates applies request body updates to the config.
func applyConfigUpdates(cfg *gitsync.Config, body *api.UpdateSyncConfigJSONRequestBody) {
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	}
	if body.Repository != nil {
		cfg.Repository = *body.Repository
	}
	if body.Branch != nil {
		cfg.Branch = *body.Branch
	}
	if body.Path != nil {
		cfg.Path = *body.Path
	}
	if body.PushEnabled != nil {
		cfg.PushEnabled = *body.PushEnabled
	}
	if body.Auth != nil {
		cfg.Auth.Type = string(body.Auth.Type)
		if body.Auth.Token != nil {
			cfg.Auth.Token = *body.Auth.Token
		}
		if body.Auth.SshKeyPath != nil {
			cfg.Auth.SSHKeyPath = *body.Auth.SshKeyPath
		}
	}
	if body.AutoSync != nil {
		cfg.AutoSync.Enabled = body.AutoSync.Enabled
		cfg.AutoSync.OnStartup = body.AutoSync.OnStartup
		cfg.AutoSync.Interval = body.AutoSync.Interval
	}
	if body.Commit != nil {
		if body.Commit.AuthorName != nil {
			cfg.Commit.AuthorName = *body.Commit.AuthorName
		}
		if body.Commit.AuthorEmail != nil {
			cfg.Commit.AuthorEmail = *body.Commit.AuthorEmail
		}
	}
}

// extractPublishOptions extracts message and force options from the request body.
func extractPublishOptions(body *api.PublishDagJSONRequestBody) (message string, force bool) {
	if body == nil {
		return "", false
	}
	if body.Message != nil {
		message = *body.Message
	}
	if body.Force != nil {
		force = *body.Force
	}
	return message, force
}

// handlePublishError handles errors from the publish operation.
func handlePublishError(err error, dagName string) (api.PublishDagResponseObject, error) {
	var conflictErr *gitsync.ConflictError
	if errors.As(err, &conflictErr) {
		return api.PublishDag409JSONResponse{
			DagId:         conflictErr.DAGID,
			RemoteCommit:  ptrOf(conflictErr.RemoteCommit),
			RemoteAuthor:  ptrOf(conflictErr.RemoteAuthor),
			RemoteMessage: ptrOf(conflictErr.RemoteMessage),
			Message:       conflictErr.Error(),
		}, nil
	}
	if gitsync.IsConflict(err) {
		return api.PublishDag409JSONResponse{
			DagId:   dagName,
			Message: err.Error(),
		}, nil
	}
	if gitsync.IsDAGNotFound(err) {
		return api.PublishDag404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: err.Error(),
		}, nil
	}
	return nil, internalError(err)
}
