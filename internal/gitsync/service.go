package gitsync

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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

	// PublishAll commits and pushes the specified DAGs.
	PublishAll(ctx context.Context, message string, dagIDs []string) (*SyncResult, error)

	// Discard discards local changes for a DAG.
	Discard(ctx context.Context, dagID string) error

	// GetStatus returns the overall sync status.
	GetStatus(ctx context.Context) (*OverallStatus, error)

	// GetDAGStatus returns the sync status for a specific DAG.
	GetDAGStatus(ctx context.Context, dagID string) (*DAGState, error)

	// GetDAGDiff returns the diff between local and remote versions of a DAG.
	GetDAGDiff(ctx context.Context, dagID string) (*DAGDiff, error)

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
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Synced    []string    `json:"synced,omitempty"`
	Modified  []string    `json:"modified,omitempty"`
	Conflicts []string    `json:"conflicts,omitempty"`
	Errors    []SyncError `json:"errors,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// SyncError represents an error during sync.
type SyncError struct {
	DAGID   string `json:"dagId,omitempty"`
	Message string `json:"message"`
}

// OverallStatus represents the overall sync status.
type OverallStatus struct {
	Enabled        bool                 `json:"enabled"`
	Repository     string               `json:"repository,omitempty"`
	Branch         string               `json:"branch,omitempty"`
	Summary        SummaryStatus        `json:"summary"`
	LastSyncAt     *time.Time           `json:"lastSyncAt,omitempty"`
	LastSyncCommit string               `json:"lastSyncCommit,omitempty"`
	LastSyncStatus string               `json:"lastSyncStatus,omitempty"`
	LastError      *string              `json:"lastError,omitempty"`
	DAGs           map[string]*DAGState `json:"dags,omitempty"`
	Counts         StatusCounts         `json:"counts"`
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

// DAGDiff represents the diff between local and remote versions of a DAG.
type DAGDiff struct {
	DAGID         string     `json:"dagId"`
	Status        SyncStatus `json:"status"`
	LocalContent  string     `json:"localContent"`
	RemoteContent string     `json:"remoteContent,omitempty"`
	RemoteCommit  string     `json:"remoteCommit,omitempty"`
	RemoteAuthor  string     `json:"remoteAuthor,omitempty"`
	RemoteMessage string     `json:"remoteMessage,omitempty"`
}

// fileExtensionForID returns the file extension for a given ID.
func fileExtensionForID(id string) string {
	if isMemoryFile(id) || isSkillFile(id) {
		return ".md"
	}
	return ".yaml"
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
	if err := s.validateEnabled(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{Timestamp: time.Now()}

	// Ensure repo is cloned and opened
	if err := s.ensureRepoReady(ctx); err != nil {
		result.Success = false
		result.Message = "Failed to prepare repository"
		result.Errors = append(result.Errors, SyncError{Message: err.Error()})
		s.updateLastSyncError(err)
		return result, err
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
	result.Success = true
	result.Message = s.buildPullMessage(pullResult.AlreadyUpToDate, syncedDAGs, conflicts)

	// Update sync state
	s.updateSuccessState(currentCommit)

	return result, nil
}

// syncFilesToDAGsDir syncs files from the repo to the DAGs directory.
func (s *serviceImpl) syncFilesToDAGsDir(_ context.Context, pullResult *PullResult) ([]string, []string, error) {
	var synced []string
	var conflicts []string

	extensions := []string{".yaml", ".yml", ".md"}
	files, err := s.gitClient.ListFiles(extensions)
	if err != nil {
		return nil, nil, err
	}

	state, _ := s.stateManager.GetState()

	// Refresh hashes to detect local modifications before checking for conflicts
	s.refreshLocalHashes(state)

	for _, file := range files {
		dagID := s.filePathToDAGID(file)

		// Only allow .md files from memory/ or skills/ directories
		if filepath.Ext(file) == ".md" && !isMemoryFile(dagID) && !isSkillFile(dagID) {
			continue
		}
		repoFilePath := s.gitClient.GetFilePath(file)
		dagFilePath := s.dagIDToFilePath(dagID)

		// Read repo file content
		repoContent, err := os.ReadFile(repoFilePath) //nolint:gosec // path constructed from internal repo
		if err != nil {
			continue
		}
		repoHash := ComputeContentHash(repoContent)

		// Check if local file exists
		localContent, err := os.ReadFile(dagFilePath) //nolint:gosec // path constructed from internal dagsDir
		dagState := state.DAGs[dagID]

		if err != nil {
			// Local file doesn't exist, create it
			if err := s.ensureDir(filepath.Dir(dagFilePath)); err != nil {
				continue
			}
			if err := os.WriteFile(dagFilePath, repoContent, 0600); err != nil {
				continue
			}
			now := time.Now()
			state.DAGs[dagID] = &DAGState{
				Status:         StatusSynced,
				Kind:           KindForDAGID(dagID),
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

		// Check for locally modified files
		if dagState != nil && dagState.Status == StatusModified {
			// Local was modified, check if remote also changed
			if dagState.LastSyncedHash != repoHash {
				// Both local and remote changed - conflict
				var remoteAuthor, remoteMessage string
				if commitInfo, err := s.gitClient.GetCommitInfo(pullResult.CurrentCommit); err == nil && commitInfo != nil {
					remoteAuthor = commitInfo.Author
					remoteMessage = commitInfo.Message
				}
				now := time.Now()
				state.DAGs[dagID] = &DAGState{
					Status:             StatusConflict,
					Kind:               KindForDAGID(dagID),
					BaseCommit:         dagState.BaseCommit,
					LastSyncedHash:     dagState.LastSyncedHash,
					LastSyncedAt:       dagState.LastSyncedAt,
					LocalHash:          localHash,
					RemoteCommit:       pullResult.CurrentCommit,
					RemoteAuthor:       remoteAuthor,
					RemoteMessage:      remoteMessage,
					ConflictDetectedAt: &now,
				}
				conflicts = append(conflicts, dagID)
			}
			// Local modified but remote unchanged - preserve local changes
			continue
		}

		// Only update local file if remote changed (and local wasn't modified)
		if localHash != repoHash {
			if err := os.WriteFile(dagFilePath, repoContent, 0600); err != nil {
				continue
			}
			now := time.Now()
			state.DAGs[dagID] = &DAGState{
				Status:         StatusSynced,
				Kind:           KindForDAGID(dagID),
				BaseCommit:     pullResult.CurrentCommit,
				LastSyncedHash: repoHash,
				LastSyncedAt:   &now,
				LocalHash:      repoHash,
			}
			synced = append(synced, dagID)
		}
	}

	// Scan for local DAGs not in the repo
	_ = s.scanLocalDAGs(state)

	if err := s.stateManager.Save(state); err != nil {
		return synced, conflicts, fmt.Errorf("failed to save state: %w", err)
	}
	return synced, conflicts, nil
}

// scanLocalDAGs scans the local DAGs directory and marks any DAGs not in state as untracked.
func (s *serviceImpl) scanLocalDAGs(state *State) error {
	extensions := map[string]bool{".yaml": true, ".yml": true}

	entries, err := os.ReadDir(s.dagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // DAGs directory doesn't exist yet
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if !extensions[ext] {
			continue
		}

		dagID := strings.TrimSuffix(entry.Name(), ext)

		// Skip if already tracked
		if _, exists := state.DAGs[dagID]; exists {
			continue
		}

		// Read local file to compute hash
		filePath := filepath.Join(s.dagsDir, entry.Name())
		content, err := os.ReadFile(filePath) //nolint:gosec // path constructed from internal dagsDir
		if err != nil {
			continue
		}

		now := time.Now()
		state.DAGs[dagID] = &DAGState{
			Status:     StatusUntracked,
			Kind:       DAGKindDAG,
			LocalHash:  ComputeContentHash(content),
			ModifiedAt: &now,
		}
	}

	// Scan memory directory for .md files
	s.scanMemoryFiles(state)

	// Scan skills directory for SKILL.md files
	s.scanSkillFiles(state)

	return nil
}

// scanMemoryFiles scans the memory directory for .md files and adds them as untracked.
func (s *serviceImpl) scanMemoryFiles(state *State) {
	memDir := filepath.Join(s.dagsDir, agentMemoryDir)

	_ = filepath.WalkDir(memDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Compute dagID relative to dagsDir, without extension
		relPath, err := filepath.Rel(s.dagsDir, path)
		if err != nil {
			return nil
		}
		dagID := strings.TrimSuffix(relPath, filepath.Ext(relPath))

		// Skip if already tracked
		if _, exists := state.DAGs[dagID]; exists {
			return nil
		}

		content, err := os.ReadFile(path) //nolint:gosec // path constructed from internal dagsDir
		if err != nil {
			return nil
		}

		now := time.Now()
		state.DAGs[dagID] = &DAGState{
			Status:     StatusUntracked,
			Kind:       DAGKindMemory,
			LocalHash:  ComputeContentHash(content),
			ModifiedAt: &now,
		}
		return nil
	})
}

// scanSkillFiles scans the skills directory for SKILL.md files and adds them as untracked.
func (s *serviceImpl) scanSkillFiles(state *State) {
	skillDir := filepath.Join(s.dagsDir, agentSkillsDir)

	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return // skills directory may not exist yet
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillMDPath := filepath.Join(skillDir, entry.Name(), "SKILL.md")
		content, err := os.ReadFile(skillMDPath) //nolint:gosec // path constructed from internal dagsDir
		if err != nil {
			continue
		}

		dagID := filepath.Join(agentSkillsDir, entry.Name(), "SKILL")
		if _, exists := state.DAGs[dagID]; exists {
			continue
		}

		now := time.Now()
		state.DAGs[dagID] = &DAGState{
			Status:     StatusUntracked,
			Kind:       DAGKindSkill,
			LocalHash:  ComputeContentHash(content),
			ModifiedAt: &now,
		}
	}
}

// refreshLocalHashes recalculates hashes for all tracked DAGs and updates status if modified.
func (s *serviceImpl) refreshLocalHashes(state *State) bool {
	changed := false
	for dagID, dagState := range state.DAGs {
		// Skip untracked (no remote to compare) and conflict (already detected)
		if dagState.Status == StatusUntracked || dagState.Status == StatusConflict {
			continue
		}

		// Read current local file
		filePath, err := s.safeDAGIDToFilePath(dagID)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(filePath) //nolint:gosec // path constructed from internal dagsDir
		if err != nil {
			// File might be deleted, skip for now
			continue
		}

		currentHash := ComputeContentHash(content)

		// Update LocalHash if changed
		if dagState.LocalHash != currentHash {
			dagState.LocalHash = currentHash
			changed = true
		}

		// Check if status should change
		if dagState.Status == StatusSynced && currentHash != dagState.LastSyncedHash {
			dagState.Status = StatusModified
			dagState.ModifiedAt = new(time.Now())
			changed = true
		} else if dagState.Status == StatusModified && currentHash == dagState.LastSyncedHash {
			// User reverted changes manually - back to synced
			dagState.Status = StatusSynced
			changed = true
		}
	}
	return changed
}

// ensureDAGKinds backfills missing kind values for backward-compatible state files.
func (s *serviceImpl) ensureDAGKinds(state *State) bool {
	changed := false
	for dagID, dagState := range state.DAGs {
		if dagState == nil || dagState.Kind != "" {
			continue
		}
		dagState.Kind = KindForDAGID(dagID)
		changed = true
	}
	return changed
}

// Publish commits and pushes a single DAG to the remote.
func (s *serviceImpl) Publish(ctx context.Context, dagID, message string, force bool) (*SyncResult, error) {
	if err := s.validatePushEnabled(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{Timestamp: time.Now()}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return nil, &DAGNotFoundError{DAGID: dagID}
	}

	if err := s.validatePublishable(dagState, dagID, force); err != nil {
		return nil, err
	}

	dagFilePath, err := s.safeDAGIDToFilePath(dagID)
	if err != nil {
		return nil, err
	}
	repoFilePath, err := s.safeDAGIDToRepoPath(dagID)
	if err != nil {
		return nil, err
	}
	repoAbsPath := s.gitClient.GetFilePath(repoFilePath)

	if err := s.gitClient.Open(); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(dagFilePath) //nolint:gosec // path constructed from internal dagsDir
	if err != nil {
		return nil, fmt.Errorf("failed to read DAG file: %w", err)
	}

	if err := s.ensureDir(filepath.Dir(repoAbsPath)); err != nil {
		return nil, err
	}

	if err := os.WriteFile(repoAbsPath, content, 0600); err != nil {
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

	if err := s.gitClient.Push(ctx); err != nil {
		return nil, err
	}

	// Update DAG state to synced
	contentHash := ComputeContentHash(content)
	state.DAGs[dagID] = s.newSyncedDAGState(dagID, commitHash, contentHash)
	s.updateSuccessStateWithCommit(state, commitHash)

	result.Success = true
	result.Message = fmt.Sprintf("Published %s", dagID)
	result.Synced = []string{dagID}

	return result, nil
}

// PublishAll commits and pushes the specified DAGs.
func (s *serviceImpl) PublishAll(ctx context.Context, message string, dagIDs []string) (*SyncResult, error) {
	if err := s.validatePushEnabled(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := &SyncResult{Timestamp: time.Now()}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	publishTargets, err := s.resolvePublishTargets(state, dagIDs)
	if err != nil {
		return nil, err
	}

	if err := s.gitClient.Open(); err != nil {
		return nil, err
	}

	// Copy files and track which succeeded
	successfulDAGs := make([]string, 0, len(publishTargets))
	stagedFiles := make([]string, 0, len(publishTargets))

	for _, dagID := range publishTargets {
		dagFilePath, err := s.safeDAGIDToFilePath(dagID)
		if err != nil {
			return nil, err
		}
		repoFilePath, err := s.safeDAGIDToRepoPath(dagID)
		if err != nil {
			return nil, err
		}
		repoAbsPath := s.gitClient.GetFilePath(repoFilePath)

		content, err := os.ReadFile(dagFilePath) //nolint:gosec // path constructed from internal dagsDir
		if err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}

		if err := s.ensureDir(filepath.Dir(repoAbsPath)); err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}

		if err := os.WriteFile(repoAbsPath, content, 0600); err != nil {
			result.Errors = append(result.Errors, SyncError{DAGID: dagID, Message: err.Error()})
			continue
		}

		successfulDAGs = append(successfulDAGs, dagID)
		stagedFiles = append(stagedFiles, repoFilePath)
	}

	// Check if any files were successfully staged
	if len(successfulDAGs) == 0 {
		return nil, fmt.Errorf("all files failed to copy: %d error(s)", len(result.Errors))
	}

	// Stage only the successful files
	wt, err := s.gitClient.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}
	for _, file := range stagedFiles {
		if _, err := wt.Add(file); err != nil {
			return nil, fmt.Errorf("failed to stage file %s: %w", file, err)
		}
	}

	// Commit staged files only (do not restage ".")
	if message == "" {
		message = fmt.Sprintf("Update %d DAG(s)", len(successfulDAGs))
	}
	commitHash, err := s.gitClient.CommitStaged(message)
	if err != nil {
		return nil, err
	}

	// Push
	if err := s.gitClient.Push(ctx); err != nil {
		return nil, err
	}

	// Update state only for successfully published DAGs
	for _, dagID := range successfulDAGs {
		dagFilePath, err := s.safeDAGIDToFilePath(dagID)
		if err != nil {
			return nil, err
		}
		content, _ := os.ReadFile(dagFilePath) //nolint:gosec // path constructed from internal dagsDir
		contentHash := ComputeContentHash(content)
		state.DAGs[dagID] = s.newSyncedDAGState(dagID, commitHash, contentHash)
		result.Synced = append(result.Synced, dagID)
	}

	s.updateSuccessStateWithCommit(state, commitHash)

	result.Success = true
	result.Message = fmt.Sprintf("Published %d DAG(s)", len(result.Synced))

	return result, nil
}

// Discard discards local changes for a DAG.
func (s *serviceImpl) Discard(_ context.Context, dagID string) error {
	if err := s.validateEnabled(); err != nil {
		return err
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

	repoFilePath, err := s.safeDAGIDToRepoPath(dagID)
	if err != nil {
		return err
	}
	dagFilePath, err := s.safeDAGIDToFilePath(dagID)
	if err != nil {
		return err
	}

	// Get content from repo
	repoContent, err := os.ReadFile(s.gitClient.GetFilePath(repoFilePath))
	if err != nil {
		return fmt.Errorf("failed to read repo file: %w", err)
	}

	// Write to DAGs directory
	if err := os.WriteFile(dagFilePath, repoContent, 0600); err != nil {
		return fmt.Errorf("failed to write DAG file: %w", err)
	}

	// Update state
	contentHash := ComputeContentHash(repoContent)
	state.DAGs[dagID] = s.newSyncedDAGState(dagID, dagState.BaseCommit, contentHash)
	_ = s.stateManager.Save(state) // Best effort - discard was successful, state will sync on next operation

	return nil
}

// GetStatus returns the overall sync status.
func (s *serviceImpl) GetStatus(_ context.Context) (*OverallStatus, error) {
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
		status.LastError = new(err.Error())
		return status, nil
	}

	// Scan for new local DAGs not yet tracked
	prevCount := len(state.DAGs)
	_ = s.scanLocalDAGs(state)
	newDAGs := len(state.DAGs) > prevCount

	// Refresh hashes for existing DAGs to detect local modifications
	hashesChanged := s.refreshLocalHashes(state)
	kindsUpdated := s.ensureDAGKinds(state)

	// Save state if anything changed (best effort - read-only operation)
	if newDAGs || hashesChanged || kindsUpdated {
		_ = s.stateManager.Save(state)
	}

	status.LastSyncAt = state.LastSyncAt
	status.LastSyncCommit = state.LastSyncCommit
	status.LastSyncStatus = state.LastSyncStatus
	status.LastError = state.LastError
	status.DAGs = state.DAGs

	status.Counts = computeStatusCounts(state.DAGs)

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
func (s *serviceImpl) GetDAGStatus(_ context.Context, dagID string) (*DAGState, error) {
	if err := s.validateEnabled(); err != nil {
		return nil, err
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return nil, &DAGNotFoundError{DAGID: dagID}
	}
	if dagState.Kind == "" {
		dagState.Kind = KindForDAGID(dagID)
		_ = s.stateManager.Save(state)
	}

	return dagState, nil
}

// GetDAGDiff returns the diff between local and remote versions of a DAG.
func (s *serviceImpl) GetDAGDiff(_ context.Context, dagID string) (*DAGDiff, error) {
	if err := s.validateEnabled(); err != nil {
		return nil, err
	}

	state, err := s.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		return nil, &DAGNotFoundError{DAGID: dagID}
	}

	localPath, err := s.safeDAGIDToFilePath(dagID)
	if err != nil {
		return nil, err
	}
	localContent, err := os.ReadFile(localPath) //nolint:gosec // path constructed from internal dagsDir
	if err != nil {
		return nil, fmt.Errorf("failed to read local file: %w", err)
	}

	diff := &DAGDiff{
		DAGID:        dagID,
		Status:       dagState.Status,
		LocalContent: string(localContent),
	}

	switch dagState.Status {
	case StatusSynced:
		diff.RemoteContent = string(localContent)
		diff.RemoteCommit = dagState.BaseCommit

	case StatusModified:
		diff.RemoteContent = s.fetchRemoteContent(dagID, dagState.BaseCommit)
		diff.RemoteCommit = dagState.BaseCommit

	case StatusConflict:
		diff.RemoteContent = s.fetchRemoteContent(dagID, dagState.RemoteCommit)
		diff.RemoteCommit = dagState.RemoteCommit
		diff.RemoteAuthor = dagState.RemoteAuthor
		diff.RemoteMessage = dagState.RemoteMessage

	case StatusUntracked:
		// No remote version for untracked files
	}

	return diff, nil
}

// fetchRemoteContent retrieves the content of a DAG file from a specific commit.
func (s *serviceImpl) fetchRemoteContent(dagID, commitHash string) string {
	if commitHash == "" {
		return ""
	}
	if err := s.gitClient.Open(); err != nil {
		return ""
	}
	repoPath, err := s.safeDAGIDToRepoPath(dagID)
	if err != nil {
		return ""
	}
	content, err := s.gitClient.GetFileContentAtCommit(repoPath, commitHash)
	if err != nil {
		return ""
	}
	return string(content)
}

// GetConfig returns the current configuration.
func (s *serviceImpl) GetConfig(_ context.Context) (*Config, error) {
	return s.cfg, nil
}

// UpdateConfig updates the configuration.
func (s *serviceImpl) UpdateConfig(_ context.Context, cfg *Config) error {
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
	state.LastError = new(err.Error())
	state.LastSyncStatus = "error"
	_ = s.stateManager.Save(state)
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

// resolvePublishTargets validates and canonicalizes DAG IDs for batch publish.
func (s *serviceImpl) resolvePublishTargets(state *State, dagIDs []string) ([]string, error) {
	if len(dagIDs) == 0 {
		return nil, &ValidationError{
			Field:   "dagIds",
			Message: "at least one DAG ID is required",
		}
	}

	resolved := make([]string, 0, len(dagIDs))
	seen := make(map[string]struct{}, len(dagIDs))
	for i, dagID := range dagIDs {
		if strings.TrimSpace(dagID) == "" {
			return nil, &ValidationError{
				Field:   fmt.Sprintf("dagIds[%d]", i),
				Message: "DAG ID cannot be empty",
			}
		}

		normalized, err := normalizeDAGID(dagID)
		if err != nil {
			return nil, err
		}
		if normalized != dagID {
			return nil, &InvalidDAGIDError{
				DAGID:  dagID,
				Reason: fmt.Sprintf("must be normalized as %q", normalized),
			}
		}

		if _, exists := seen[dagID]; exists {
			continue
		}
		seen[dagID] = struct{}{}

		dagState, exists := state.DAGs[dagID]
		if !exists {
			return nil, &ValidationError{
				Field:   "dagIds",
				Message: fmt.Sprintf("DAG %q is not tracked by git sync", dagID),
			}
		}

		switch dagState.Status {
		case StatusModified, StatusUntracked:
			resolved = append(resolved, dagID)
		case StatusConflict:
			return nil, &ValidationError{
				Field:   "dagIds",
				Message: fmt.Sprintf("DAG %q has conflicts and cannot be batch-published", dagID),
			}
		case StatusSynced:
			return nil, &ValidationError{
				Field:   "dagIds",
				Message: fmt.Sprintf("DAG %q has no local changes", dagID),
			}
		default:
			return nil, &ValidationError{
				Field:   "dagIds",
				Message: fmt.Sprintf("DAG %q is in unsupported status %q", dagID, dagState.Status),
			}
		}
	}

	if len(resolved) == 0 {
		return nil, &ValidationError{
			Field:   "dagIds",
			Message: "no publishable DAG IDs provided",
		}
	}

	sort.Strings(resolved)
	return resolved, nil
}

func (s *serviceImpl) dagIDToFilePath(dagID string) string {
	// Decode if URL encoded
	decoded, err := url.PathUnescape(dagID)
	if err == nil {
		dagID = decoded
	}
	ext := fileExtensionForID(dagID)
	return filepath.Join(s.dagsDir, dagID+ext)
}

func (s *serviceImpl) dagIDToRepoPath(dagID string) string {
	// Decode if URL encoded
	decoded, err := url.PathUnescape(dagID)
	if err == nil {
		dagID = decoded
	}
	ext := fileExtensionForID(dagID)
	if s.cfg.Path != "" {
		return filepath.Join(s.cfg.Path, dagID+ext)
	}
	return dagID + ext
}

func decodeDAGID(dagID string) (string, error) {
	decoded, err := url.PathUnescape(strings.TrimSpace(dagID))
	if err != nil {
		return "", &InvalidDAGIDError{
			DAGID:  dagID,
			Reason: "contains invalid URL escape sequence",
		}
	}
	return decoded, nil
}

func normalizeDAGID(dagID string) (string, error) {
	decoded, err := decodeDAGID(dagID)
	if err != nil {
		return "", err
	}
	if decoded == "" {
		return "", &InvalidDAGIDError{DAGID: dagID, Reason: "cannot be empty"}
	}
	if filepath.IsAbs(decoded) {
		return "", &InvalidDAGIDError{DAGID: dagID, Reason: "absolute paths are not allowed"}
	}

	clean := filepath.Clean(decoded)
	if clean == "." || clean == ".." {
		return "", &InvalidDAGIDError{DAGID: dagID, Reason: "must point to a DAG ID, not current/parent directory"}
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", &InvalidDAGIDError{DAGID: dagID, Reason: "path traversal is not allowed"}
	}

	return clean, nil
}

func safeJoinWithinBase(baseDir, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", &InvalidDAGIDError{
			DAGID:  relativePath,
			Reason: "absolute paths are not allowed",
		}
	}

	cleanRel := filepath.Clean(relativePath)
	if cleanRel == "." || cleanRel == ".." {
		return "", &InvalidDAGIDError{
			DAGID:  relativePath,
			Reason: "must be a valid relative path",
		}
	}
	if strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", &InvalidDAGIDError{
			DAGID:  relativePath,
			Reason: "path traversal is not allowed",
		}
	}

	fullPath := filepath.Join(baseDir, cleanRel)
	relToBase, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return "", &InvalidDAGIDError{
			DAGID:  relativePath,
			Reason: "cannot resolve path safely",
		}
	}
	if relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) {
		return "", &InvalidDAGIDError{
			DAGID:  relativePath,
			Reason: "path escapes allowed base directory",
		}
	}

	return fullPath, nil
}

func (s *serviceImpl) safeDAGIDToFilePath(dagID string) (string, error) {
	normalized, err := normalizeDAGID(dagID)
	if err != nil {
		return "", err
	}
	ext := fileExtensionForID(normalized)
	return safeJoinWithinBase(s.dagsDir, normalized+ext)
}

func (s *serviceImpl) safeDAGIDToRepoPath(dagID string) (string, error) {
	normalized, err := normalizeDAGID(dagID)
	if err != nil {
		return "", err
	}

	ext := fileExtensionForID(normalized)
	repoPath := normalized + ext
	if s.cfg.Path != "" {
		repoPath = filepath.Join(s.cfg.Path, repoPath)
	}

	safePath, err := safeJoinWithinBase(s.gitClient.repoPath, repoPath)
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(s.gitClient.repoPath, safePath)
	if err != nil {
		return "", &InvalidDAGIDError{
			DAGID:  dagID,
			Reason: "cannot resolve repository path",
		}
	}
	return relPath, nil
}

func (s *serviceImpl) ensureDir(dir string) error {
	return os.MkdirAll(dir, 0750)
}

// validateEnabled checks if git sync is enabled and configured.
func (s *serviceImpl) validateEnabled() error {
	if !s.cfg.Enabled {
		return ErrNotEnabled
	}
	if !s.cfg.IsValid() {
		return ErrNotConfigured
	}
	return nil
}

// validatePushEnabled checks if push operations are allowed.
func (s *serviceImpl) validatePushEnabled() error {
	if !s.cfg.Enabled {
		return ErrNotEnabled
	}
	if !s.cfg.PushEnabled {
		return ErrPushDisabled
	}
	return nil
}

// validatePublishable checks if a DAG can be published.
func (s *serviceImpl) validatePublishable(dagState *DAGState, dagID string, force bool) error {
	if dagState.Status == StatusConflict && !force {
		return &ConflictError{
			DAGID:         dagID,
			RemoteCommit:  dagState.RemoteCommit,
			RemoteAuthor:  dagState.RemoteAuthor,
			RemoteMessage: dagState.RemoteMessage,
		}
	}
	if dagState.Status == StatusSynced {
		return ErrNoChanges
	}
	return nil
}

// ensureRepoReady ensures the repository is cloned and opened.
func (s *serviceImpl) ensureRepoReady(ctx context.Context) error {
	if !s.gitClient.IsCloned() {
		return s.gitClient.Clone(ctx)
	}
	return s.gitClient.Open()
}

// newSyncedDAGState creates a new DAGState in synced status.
func (s *serviceImpl) newSyncedDAGState(dagID, commitHash, contentHash string) *DAGState {
	now := time.Now()
	return &DAGState{
		Status:         StatusSynced,
		Kind:           KindForDAGID(dagID),
		BaseCommit:     commitHash,
		LastSyncedHash: contentHash,
		LastSyncedAt:   &now,
		LocalHash:      contentHash,
	}
}

// updateSuccessState updates the global sync state after a successful pull.
func (s *serviceImpl) updateSuccessState(commitHash string) {
	state, err := s.stateManager.GetState()
	if err != nil {
		// Initialize new state if loading fails
		state = &State{
			Version: 1,
			DAGs:    make(map[string]*DAGState),
		}
	}
	s.updateSuccessStateWithCommit(state, commitHash)
}

// updateSuccessStateWithCommit updates and saves the state after a successful sync.
func (s *serviceImpl) updateSuccessStateWithCommit(state *State, commitHash string) {
	state.LastSyncAt = new(time.Now())
	state.LastSyncCommit = commitHash
	state.LastSyncStatus = "success"
	state.LastError = nil
	state.Repository = s.cfg.Repository
	state.Branch = s.cfg.Branch
	_ = s.stateManager.Save(state) // Best effort - state will be recovered on next load
}

// buildPullMessage constructs the result message for a pull operation.
func (s *serviceImpl) buildPullMessage(alreadyUpToDate bool, synced, conflicts []string) string {
	if len(conflicts) > 0 {
		return fmt.Sprintf("Pulled with %d conflict(s)", len(conflicts))
	}
	if alreadyUpToDate {
		return "Already up to date"
	}
	return fmt.Sprintf("Synced %d DAG(s)", len(synced))
}

// computeStatusCounts computes the counts for each DAG status.
func computeStatusCounts(dags map[string]*DAGState) StatusCounts {
	var counts StatusCounts
	for _, dagState := range dags {
		switch dagState.Status {
		case StatusSynced:
			counts.Synced++
		case StatusModified:
			counts.Modified++
		case StatusUntracked:
			counts.Untracked++
		case StatusConflict:
			counts.Conflict++
		}
	}
	return counts
}
