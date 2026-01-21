// Copyright (C) 2025 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package gitsync

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service defines the interface for Git sync operations.
type Service interface {
	// Pull fetches and merges changes from the remote repository.
	Pull(ctx context.Context) (*SyncResult, error)

	// Publish commits and pushes a single DAG to the remote.
	Publish(ctx context.Context, dagID, message string, force bool) (*SyncResult, error)

	// PublishAll commits and pushes all modified DAGs.
	PublishAll(ctx context.Context, message string) (*SyncResult, error)

	// Discard discards local changes for a DAG.
	Discard(ctx context.Context, dagID string) error

	// GetStatus returns the overall sync status.
	GetStatus(ctx context.Context) (*OverallStatus, error)

	// GetDAGStatus returns the sync status for a specific DAG.
	GetDAGStatus(ctx context.Context, dagID string) (*DAGState, error)

	// GetConfig returns the current configuration.
	GetConfig(ctx context.Context) (*Config, error)

	// UpdateConfig updates the configuration.
	UpdateConfig(ctx context.Context, cfg *Config) error

	// TestConnection tests the connection to the remote repository.
	TestConnection(ctx context.Context) (*ConnectionResult, error)

	// Start starts the auto-sync background worker.
	Start(ctx context.Context) error

	// Stop stops the auto-sync background worker.
	Stop() error
}

// SyncResult represents the result of a sync operation.
type SyncResult struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message,omitempty"`
	Synced    []string     `json:"synced,omitempty"`
	Modified  []string     `json:"modified,omitempty"`
	Conflicts []string     `json:"conflicts,omitempty"`
	Errors    []SyncError  `json:"errors,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// SyncError represents an error during sync.
type SyncError struct {
	DAGID   string `json:"dagId,omitempty"`
	Message string `json:"message"`
}

// OverallStatus represents the overall sync status.
type OverallStatus struct {
	Enabled        bool                  `json:"enabled"`
	Repository     string                `json:"repository,omitempty"`
	Branch         string                `json:"branch,omitempty"`
	Summary        SummaryStatus         `json:"summary"`
	LastSyncAt     *time.Time            `json:"lastSyncAt,omitempty"`
	LastSyncCommit string                `json:"lastSyncCommit,omitempty"`
	LastSyncStatus string                `json:"lastSyncStatus,omitempty"`
	LastError      *string               `json:"lastError,omitempty"`
	DAGs           map[string]*DAGState  `json:"dags,omitempty"`
	Counts         StatusCounts          `json:"counts"`
}

// SummaryStatus represents the summary status for the header badge.
type SummaryStatus string

const (
	SummarySynced   SummaryStatus = "synced"
	SummaryPending  SummaryStatus = "pending"
	SummaryConflict SummaryStatus = "conflict"
	SummaryError    SummaryStatus = "error"
)

// StatusCounts contains counts for each status type.
type StatusCounts struct {
	Synced    int `json:"synced"`
	Modified  int `json:"modified"`
	Untracked int `json:"untracked"`
	Conflict  int `json:"conflict"`
}

// ConnectionResult represents the result of a connection test.
type ConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// serviceImpl implements the Service interface.
type serviceImpl struct {
	cfg          *Config
	dagsDir      string
	dataDir      string
	stateManager *StateManager
	gitClient    *GitClient
	mu           sync.Mutex
	stopCh       chan struct{}
	running      bool
}

// NewService creates a new Git sync service.
func NewService(cfg *Config, dagsDir, dataDir string) Service {
	repoPath := filepath.Join(dataDir, "gitsync", "repo")
	return &serviceImpl{
		cfg:          cfg,
		dagsDir:      dagsDir,
		dataDir:      dataDir,
		stateManager: NewStateManager(dataDir),
		gitClient:    NewGitClient(cfg, repoPath),
	}
}

// Pull fetches and merges changes from the remote repository.
func (s *serviceImpl) Pull(ctx context.Context) (*SyncResult, error) {
	if !s.cfg.Enabled {
		return nil, ErrNotEnabled
	}
	if !s.cfg.IsValid() {
		return nil, ErrNotConfigured
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{
		Timestamp: time.Now(),
	}

	// Ensure repo is cloned
	if !s.gitClient.IsCloned() {
		if err := s.gitClient.Clone(ctx); err != nil {
			result.Success = false
			result.Message = "Failed to clone repository"
			result.Errors = append(result.Errors, SyncError{Message: err.Error()})
			s.updateLastSyncError(err)
			return result, err
		}
	} else {
		if err := s.gitClient.Open(); err != nil {
			return nil, err
		}
	}

	// Fetch and pull
	pullResult, err := s.gitClient.Pull(ctx)
	if err != nil {
		result.Success = false
		result.Message = "Failed to pull changes"
		result.Errors = append(result.Errors, SyncError{Message: err.Error()})
		s.updateLastSyncError(err)
		return result, err
	}

	// Get current commit
	currentCommit, _ := s.gitClient.GetHeadCommit()

	// Sync files to DAGs directory
	syncedDAGs, conflicts, err := s.syncFilesToDAGsDir(ctx, pullResult)
	if err != nil {
		result.Success = false
		result.Message = "Failed to sync files"
		result.Errors = append(result.Errors, SyncError{Message: err.Error()})
		s.updateLastSyncError(err)
		return result, err
	}

	result.Synced = syncedDAGs
	result.Conflicts = conflicts

	// Update state
	state, _ := s.stateManager.GetState()
	now := time.Now()
	state.LastSyncAt = &now
	state.LastSyncCommit = currentCommit
	state.LastSyncStatus = "success"
	state.LastError = nil
	state.Repository = s.cfg.Repository
	state.Branch = s.cfg.Branch
	s.stateManager.Save(state)

	result.Success = true
	switch {
	case len(conflicts) > 0:
		result.Message = fmt.Sprintf("Pulled with %d conflict(s)", len(conflicts))
	case pullResult.AlreadyUpToDate:
		result.Message = "Already up to date"
	default:
		result.Message = fmt.Sprintf("Synced %d DAG(s)", len(syncedDAGs))
	}

	return result, nil
}

// syncFilesToDAGsDir syncs files from the repo to the DAGs directory.
func (s *serviceImpl) syncFilesToDAGsDir(ctx context.Context, pullResult *PullResult) ([]string, []string, error) {
	var synced []string
	var conflicts []string

	extensions := []string{".yaml", ".yml"}
	files, err := s.gitClient.ListFiles(extensions)
	if err != nil {
		return nil, nil, err
	}

	state, _ := s.stateManager.GetState()

	for _, file := range files {
		dagID := s.filePathToDAGID(file)
		repoFilePath := s.gitClient.GetFilePath(file)
		dagFilePath := s.dagIDToFilePath(dagID)

		// Read repo file content
		repoContent, err := os.ReadFile(repoFilePath)
		if err != nil {
			continue
		}
		repoHash := ComputeContentHash(repoContent)

		// Check if local file exists
		localContent, err := os.ReadFile(dagFilePath)
		dagState := state.DAGs[dagID]

		if err != nil {
			// Local file doesn't exist, create it
			if err := s.ensureDir(filepath.Dir(dagFilePath)); err != nil {
				continue
			}
			if err := os.WriteFile(dagFilePath, repoContent, 0644); err != nil {
				continue
			}
			now := time.Now()
			state.DAGs[dagID] = &DAGState{
				Status:         StatusSynced,
				BaseCommit:     pullResult.CurrentCommit,
				LastSyncedHash: repoHash,
				LastSyncedAt:   &now,
				LocalHash:      repoHash,
				ModifiedAt:     &now, // Added ModifiedAt for new files
			}
			synced = append(synced, dagID)
			continue
		}

		localHash := ComputeContentHash(localContent)

		// Check for conflict
		if dagState != nil && dagState.Status == StatusModified {
			// Local was modified, check if remote also changed
			if dagState.LastSyncedHash != repoHash {
				// Both local and remote changed - conflict
				commitInfo, _ := s.gitClient.GetCommitInfo(pullResult.CurrentCommit)
				now := time.Now()
				state.DAGs[dagID] = &DAGState{
					Status:             StatusConflict,
					BaseCommit:         dagState.BaseCommit,
					LastSyncedHash:     dagState.LastSyncedHash,
					LastSyncedAt:       dagState.LastSyncedAt,
					LocalHash:          localHash,
					RemoteCommit:       pullResult.CurrentCommit,
					RemoteAuthor:       commitInfo.Author,
					RemoteMessage:      commitInfo.Message,
					ConflictDetectedAt: &now,
				}
				conflicts = append(conflicts, dagID)
				continue
			}
		}

		// No conflict, update local file if remote changed
		if localHash != repoHash {
			if err := os.WriteFile(dagFilePath, repoContent, 0644); err != nil {
				continue
			}
			now := time.Now()
			state.DAGs[dagID] = &DAGState{
				Status:         StatusSynced,
				BaseCommit:     pullResult.CurrentCommit,
				LastSyncedHash: repoHash,
				LastSyncedAt:   &now,
				LocalHash:      repoHash,
			}
			synced = append(synced, dagID)
		}
	}

	s.stateManager.Save(state)
	return synced, conflicts, nil
}

// Publish commits and pushes a single DAG to the remote.
func (s *serviceImpl) Publish(ctx context.Context, dagID, message string, force bool) (*SyncResult, error) {
	if !s.cfg.Enabled {
		return nil, ErrNotEnabled
	}
	if !s.cfg.PushEnabled {
		return nil, ErrPushDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{
		Timestamp: time.Now(),
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return nil, &DAGNotFoundError{DAGID: dagID}
	}

	// Check for conflict
	if dagState.Status == StatusConflict && !force {
		return nil, &ConflictError{
			DAGID:         dagID,
			RemoteCommit:  dagState.RemoteCommit,
			RemoteAuthor:  dagState.RemoteAuthor,
			RemoteMessage: dagState.RemoteMessage,
		}
	}

	// Check if there are changes to publish
	if dagState.Status == StatusSynced {
		return nil, ErrNoChanges
	}

	// Open repo
	if err := s.gitClient.Open(); err != nil {
		return nil, err
	}

	// Copy file to repo
	dagFilePath := s.dagIDToFilePath(dagID)
	repoFilePath := s.dagIDToRepoPath(dagID)

	content, err := os.ReadFile(dagFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read DAG file: %w", err)
	}

	if err := s.ensureDir(filepath.Dir(s.gitClient.GetFilePath(repoFilePath))); err != nil {
		return nil, err
	}

	if err := os.WriteFile(s.gitClient.GetFilePath(repoFilePath), content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write to repo: %w", err)
	}

	// Commit
	if message == "" {
		message = fmt.Sprintf("Update %s", dagID)
	}
	commitHash, err := s.gitClient.AddAndCommit(repoFilePath, message)
	if err != nil {
		return nil, err
	}

	// Push
	if err := s.gitClient.Push(ctx); err != nil {
		return nil, err
	}

	// Update state
	now := time.Now()
	contentHash := ComputeContentHash(content)
	state.DAGs[dagID] = &DAGState{
		Status:         StatusSynced,
		BaseCommit:     commitHash,
		LastSyncedHash: contentHash,
		LastSyncedAt:   &now,
		LocalHash:      contentHash,
	}
	state.LastSyncAt = &now
	state.LastSyncCommit = commitHash
	state.LastSyncStatus = "success"
	s.stateManager.Save(state)

	result.Success = true
	result.Message = fmt.Sprintf("Published %s", dagID)
	result.Synced = []string{dagID}

	return result, nil
}

// PublishAll commits and pushes all modified DAGs.
func (s *serviceImpl) PublishAll(ctx context.Context, message string) (*SyncResult, error) {
	if !s.cfg.Enabled {
		return nil, ErrNotEnabled
	}
	if !s.cfg.PushEnabled {
		return nil, ErrPushDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{
		Timestamp: time.Now(),
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	// Find all modified DAGs (skip conflicts)
	var modifiedDAGs []string
	for dagID, dagState := range state.DAGs {
		if dagState.Status == StatusModified || dagState.Status == StatusUntracked {
			modifiedDAGs = append(modifiedDAGs, dagID)
		}
	}

	if len(modifiedDAGs) == 0 {
		return nil, ErrNoChanges
	}

	// Open repo
	if err := s.gitClient.Open(); err != nil {
		return nil, err
	}

	// Copy and stage all files
	for _, dagID := range modifiedDAGs {
		dagFilePath := s.dagIDToFilePath(dagID)
		repoFilePath := s.dagIDToRepoPath(dagID)

		content, err := os.ReadFile(dagFilePath)
		if err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}

		if err := s.ensureDir(filepath.Dir(s.gitClient.GetFilePath(repoFilePath))); err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}

		if err := os.WriteFile(s.gitClient.GetFilePath(repoFilePath), content, 0644); err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}
	}

	// Commit all
	if message == "" {
		message = fmt.Sprintf("Update %d DAG(s)", len(modifiedDAGs))
	}
	commitHash, err := s.gitClient.AddAndCommit(".", message)
	if err != nil {
		return nil, err
	}

	// Push
	if err := s.gitClient.Push(ctx); err != nil {
		return nil, err
	}

	// Update state for all published DAGs
	now := time.Now()
	for _, dagID := range modifiedDAGs {
		dagFilePath := s.dagIDToFilePath(dagID)
		content, _ := os.ReadFile(dagFilePath)
		contentHash := ComputeContentHash(content)

		state.DAGs[dagID] = &DAGState{
			Status:         StatusSynced,
			BaseCommit:     commitHash,
			LastSyncedHash: contentHash,
			LastSyncedAt:   &now,
			LocalHash:      contentHash,
		}
		result.Synced = append(result.Synced, dagID)
	}

	state.LastSyncAt = &now
	state.LastSyncCommit = commitHash
	state.LastSyncStatus = "success"
	s.stateManager.Save(state)

	result.Success = true
	result.Message = fmt.Sprintf("Published %d DAG(s)", len(result.Synced))

	return result, nil
}

// Discard discards local changes for a DAG.
func (s *serviceImpl) Discard(ctx context.Context, dagID string) error {
	if !s.cfg.Enabled {
		return ErrNotEnabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.stateManager.GetState()
	if err != nil {
		return err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return &DAGNotFoundError{DAGID: dagID}
	}

	// Open repo
	if err := s.gitClient.Open(); err != nil {
		return err
	}

	// Get content from repo
	repoFilePath := s.dagIDToRepoPath(dagID)
	repoContent, err := os.ReadFile(s.gitClient.GetFilePath(repoFilePath))
	if err != nil {
		return fmt.Errorf("failed to read repo file: %w", err)
	}

	// Write to DAGs directory
	dagFilePath := s.dagIDToFilePath(dagID)
	if err := os.WriteFile(dagFilePath, repoContent, 0644); err != nil {
		return fmt.Errorf("failed to write DAG file: %w", err)
	}

	// Update state
	now := time.Now()
	contentHash := ComputeContentHash(repoContent)
	state.DAGs[dagID] = &DAGState{
		Status:         StatusSynced,
		BaseCommit:     dagState.BaseCommit,
		LastSyncedHash: contentHash,
		LastSyncedAt:   &now,
		LocalHash:      contentHash,
	}
	s.stateManager.Save(state)

	return nil
}

// GetStatus returns the overall sync status.
func (s *serviceImpl) GetStatus(ctx context.Context) (*OverallStatus, error) {
	status := &OverallStatus{
		Enabled: s.cfg.Enabled,
	}

	if !s.cfg.Enabled {
		return status, nil
	}

	status.Repository = s.cfg.Repository
	status.Branch = s.cfg.Branch

	state, err := s.stateManager.GetState()
	if err != nil {
		status.Summary = SummaryError
		errMsg := err.Error()
		status.LastError = &errMsg
		return status, nil
	}

	status.LastSyncAt = state.LastSyncAt
	status.LastSyncCommit = state.LastSyncCommit
	status.LastSyncStatus = state.LastSyncStatus
	status.LastError = state.LastError
	status.DAGs = state.DAGs

	// Compute counts and summary
	for _, dagState := range state.DAGs {
		switch dagState.Status {
		case StatusSynced:
			status.Counts.Synced++
		case StatusModified:
			status.Counts.Modified++
		case StatusUntracked:
			status.Counts.Untracked++
		case StatusConflict:
			status.Counts.Conflict++
		}
	}

	// Determine summary status
	if status.Counts.Conflict > 0 {
		status.Summary = SummaryConflict
	} else if status.Counts.Modified > 0 || status.Counts.Untracked > 0 {
		status.Summary = SummaryPending
	} else {
		status.Summary = SummarySynced
	}

	if state.LastError != nil {
		status.Summary = SummaryError
	}

	return status, nil
}

// GetDAGStatus returns the sync status for a specific DAG.
func (s *serviceImpl) GetDAGStatus(ctx context.Context, dagID string) (*DAGState, error) {
	if !s.cfg.Enabled {
		return nil, ErrNotEnabled
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return nil, &DAGNotFoundError{DAGID: dagID}
	}

	return dagState, nil
}

// GetConfig returns the current configuration.
func (s *serviceImpl) GetConfig(ctx context.Context) (*Config, error) {
	return s.cfg, nil
}

// UpdateConfig updates the configuration.
func (s *serviceImpl) UpdateConfig(ctx context.Context, cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg = cfg
	s.gitClient = NewGitClient(cfg, filepath.Join(s.dataDir, "gitsync", "repo"))

	return nil
}

// TestConnection tests the connection to the remote repository.
func (s *serviceImpl) TestConnection(ctx context.Context) (*ConnectionResult, error) {
	if !s.cfg.Enabled {
		return &ConnectionResult{
			Success: false,
			Error:   "Git sync is not enabled",
		}, nil
	}

	if !s.cfg.IsValid() {
		return &ConnectionResult{
			Success: false,
			Error:   "Git sync configuration is invalid",
		}, nil
	}

	err := s.gitClient.TestConnection(ctx)
	if err != nil {
		return &ConnectionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ConnectionResult{
		Success: true,
		Message: "Connection successful",
	}, nil
}

// Start starts the auto-sync background worker.
func (s *serviceImpl) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	// Initial sync on startup
	if s.cfg.AutoSync.OnStartup {
		_, _ = s.Pull(ctx)
	}

	// Start periodic sync if interval > 0
	if s.cfg.AutoSync.Enabled && s.cfg.AutoSync.Interval > 0 {
		go s.runAutoSync(ctx)
	}

	return nil
}

// Stop stops the auto-sync background worker.
func (s *serviceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopCh)
	s.running = false

	return nil
}

// runAutoSync runs the auto-sync loop.
func (s *serviceImpl) runAutoSync(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.AutoSync.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Pull(ctx); err != nil {
				// We don't return error here to keep the loop running,
				// but the error is already updated in s.updateLastSyncError via s.Pull
				fmt.Fprintf(os.Stderr, "Auto-sync failed: %v\n", err)
			}
		}
	}
}

// Helper methods

func (s *serviceImpl) updateLastSyncError(err error) {
	state, _ := s.stateManager.GetState()
	errMsg := err.Error()
	state.LastError = &errMsg
	state.LastSyncStatus = "error"
	s.stateManager.Save(state)
}

func (s *serviceImpl) filePathToDAGID(filePath string) string {
	// Remove path prefix if configured
	if s.cfg.Path != "" {
		filePath = strings.TrimPrefix(filePath, s.cfg.Path+"/")
	}
	// Remove extension
	ext := filepath.Ext(filePath)
	dagID := strings.TrimSuffix(filePath, ext)
	// URL encode for safety
	return dagID
}

func (s *serviceImpl) dagIDToFilePath(dagID string) string {
	// Decode if URL encoded
	decoded, err := url.PathUnescape(dagID)
	if err == nil {
		dagID = decoded
	}
	return filepath.Join(s.dagsDir, dagID+".yaml")
}

func (s *serviceImpl) dagIDToRepoPath(dagID string) string {
	// Decode if URL encoded
	decoded, err := url.PathUnescape(dagID)
	if err == nil {
		dagID = decoded
	}
	if s.cfg.Path != "" {
		return filepath.Join(s.cfg.Path, dagID+".yaml")
	}
	return dagID + ".yaml"
}

func (s *serviceImpl) ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// MarkDAGModified marks a DAG as modified (called when DAG is saved locally).
func (s *serviceImpl) MarkDAGModified(dagID string) error {
	if !s.cfg.Enabled {
		return nil
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return err
	}

	dagFilePath := s.dagIDToFilePath(dagID)
	content, err := os.ReadFile(dagFilePath)
	if err != nil {
		return err
	}

	currentHash := ComputeContentHash(content)
	dagState := state.DAGs[dagID]

	if dagState == nil {
		// New DAG, mark as untracked
		now := time.Now()
		state.DAGs[dagID] = &DAGState{
			Status:     StatusUntracked,
			LocalHash:  currentHash,
			ModifiedAt: &now,
		}
	} else if dagState.LastSyncedHash != currentHash {
		// Existing DAG modified
		now := time.Now()
		dagState.Status = StatusModified
		dagState.LocalHash = currentHash
		dagState.ModifiedAt = &now
	}

	return s.stateManager.Save(state)
}
