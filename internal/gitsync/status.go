// Copyright (C) 2025 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package gitsync

import (
	"os"
	"path/filepath"
	"strings"
)

// StatusComputer computes the sync status for DAGs.
type StatusComputer struct {
	cfg          *Config
	dagsDir      string
	stateManager *StateManager
	gitClient    *GitClient
}

// NewStatusComputer creates a new status computer.
func NewStatusComputer(cfg *Config, dagsDir, dataDir string) *StatusComputer {
	repoPath := filepath.Join(dataDir, "gitsync", "repo")
	return &StatusComputer{
		cfg:          cfg,
		dagsDir:      dagsDir,
		stateManager: NewStateManager(dataDir),
		gitClient:    NewGitClient(cfg, repoPath),
	}
}

// RefreshStatus refreshes the sync status for all DAGs.
// This compares local files with the stored state to detect modifications.
func (c *StatusComputer) RefreshStatus() error {
	if !c.cfg.Enabled {
		return nil
	}

	state, err := c.stateManager.GetState()
	if err != nil {
		return err
	}

	// Scan local DAGs directory
	localDAGs, err := c.scanLocalDAGs()
	if err != nil {
		return err
	}

	// Check each local DAG
	for dagID, localPath := range localDAGs {
		content, err := os.ReadFile(localPath)
		if err != nil {
			continue
		}
		currentHash := ComputeContentHash(content)

		dagState := state.DAGs[dagID]
		if dagState == nil {
			// New local DAG, mark as untracked
			state.DAGs[dagID] = &DAGState{
				Status:    StatusUntracked,
				LocalHash: currentHash,
			}
			continue
		}

		// Skip conflict state (must be resolved explicitly)
		if dagState.Status == StatusConflict {
			dagState.LocalHash = currentHash
			continue
		}

		// Check if modified
		if dagState.LastSyncedHash != currentHash {
			dagState.Status = StatusModified
			dagState.LocalHash = currentHash
		} else {
			dagState.Status = StatusSynced
			dagState.LocalHash = currentHash
		}
	}

	// Check for DAGs that exist in state but not locally (deleted)
	for dagID := range state.DAGs {
		if _, exists := localDAGs[dagID]; !exists {
			// DAG was deleted locally
			delete(state.DAGs, dagID)
		}
	}

	return c.stateManager.Save(state)
}

// scanLocalDAGs scans the DAGs directory and returns a map of DAG ID to file path.
func (c *StatusComputer) scanLocalDAGs() (map[string]string, error) {
	dags := make(map[string]string)

	err := filepath.WalkDir(c.dagsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Compute DAG ID
		relPath, err := filepath.Rel(c.dagsDir, path)
		if err != nil {
			return nil
		}

		dagID := strings.TrimSuffix(relPath, ext)
		dags[dagID] = path

		return nil
	})

	if err != nil {
		return nil, err
	}

	return dags, nil
}

// GetDAGStatusByPath returns the sync status for a DAG by its file path.
func (c *StatusComputer) GetDAGStatusByPath(filePath string) (*DAGState, error) {
	if !c.cfg.Enabled {
		return nil, ErrNotEnabled
	}

	// Convert path to DAG ID
	relPath, err := filepath.Rel(c.dagsDir, filePath)
	if err != nil {
		return nil, err
	}
	dagID := strings.TrimSuffix(relPath, filepath.Ext(relPath))

	state, err := c.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	dagState := state.DAGs[dagID]
	if dagState == nil {
		// Compute current state
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		return &DAGState{
			Status:    StatusUntracked,
			LocalHash: ComputeContentHash(content),
		}, nil
	}

	return dagState, nil
}

// ComputeDAGStatus computes the current status for a specific DAG.
func (c *StatusComputer) ComputeDAGStatus(dagID string) (*DAGState, error) {
	if !c.cfg.Enabled {
		return nil, ErrNotEnabled
	}

	state, err := c.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	// Read current local file
	filePath := filepath.Join(c.dagsDir, dagID+".yaml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Check yml extension
			filePath = filepath.Join(c.dagsDir, dagID+".yml")
			content, err = os.ReadFile(filePath)
			if err != nil {
				return nil, &DAGNotFoundError{DAGID: dagID}
			}
		} else {
			return nil, err
		}
	}

	currentHash := ComputeContentHash(content)
	dagState := state.DAGs[dagID]

	if dagState == nil {
		return &DAGState{
			Status:    StatusUntracked,
			LocalHash: currentHash,
		}, nil
	}

	// Update local hash
	dagState.LocalHash = currentHash

	// Recompute status if not in conflict
	if dagState.Status != StatusConflict {
		if dagState.LastSyncedHash == currentHash {
			dagState.Status = StatusSynced
		} else {
			dagState.Status = StatusModified
		}
	}

	return dagState, nil
}

// GetAllDAGStatuses returns the sync status for all DAGs.
func (c *StatusComputer) GetAllDAGStatuses() (map[string]*DAGState, error) {
	if !c.cfg.Enabled {
		return nil, ErrNotEnabled
	}

	// Refresh status first
	if err := c.RefreshStatus(); err != nil {
		return nil, err
	}

	state, err := c.stateManager.GetState()
	if err != nil {
		return nil, err
	}

	return state.DAGs, nil
}
