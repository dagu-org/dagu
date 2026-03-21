// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
)

const (
	procFileVersion   = 1
	procFilePrefix    = "proc_"
	procFileExt       = ".proc"
	heartbeatSize     = 8
	dateTimeFormatUTC = "20060102_150405"
)

var (
	errInvalidProcFile = errors.New("invalid proc file")
	procFileRegex      = regexp.MustCompile(`^proc_(\d{8}_\d{6}Z)_([0-9a-f]+)_([0-9a-f]+)\.proc$`)
)

type procDiskMeta struct {
	Version      int    `json:"version"`
	DAGName      string `json:"dag_name"`
	DAGRunID     string `json:"dag_run_id"`
	AttemptID    string `json:"attempt_id"`
	RootName     string `json:"root_name,omitempty"`
	RootDAGRunID string `json:"root_dag_run_id,omitempty"`
	StartedAt    int64  `json:"started_at"`
}

type procFileName struct {
	createdAt time.Time
	dagRunID  string
	attemptID string
}

func validateProcMeta(meta exec.ProcMeta) error {
	if meta.Name == "" {
		return fmt.Errorf("proc meta name is required")
	}
	if err := exec.ValidateDAGRunID(meta.DAGRunID); err != nil {
		return fmt.Errorf("invalid proc meta dag run id: %w", err)
	}
	if meta.AttemptID == "" {
		return fmt.Errorf("proc meta attempt id is required")
	}
	if !reSafeID.MatchString(meta.AttemptID) {
		return fmt.Errorf("proc meta attempt id must only contain alphanumeric characters, dashes, and underscores")
	}
	if meta.StartedAt <= 0 {
		return fmt.Errorf("proc meta started at must be > 0")
	}
	if (meta.RootName == "") != (meta.RootDAGRunID == "") {
		return fmt.Errorf("proc meta root name and root dag run id must both be set or both be empty")
	}
	if meta.RootDAGRunID != "" {
		if err := exec.ValidateDAGRunID(meta.RootDAGRunID); err != nil {
			return fmt.Errorf("invalid proc meta root dag run id: %w", err)
		}
	}
	return nil
}

var reSafeID = regexp.MustCompile(`^[-a-zA-Z0-9_]+$`)

func procFilePath(baseDir string, t exec.TimeInUTC, meta exec.ProcMeta) string {
	timestamp := t.Format(dateTimeFormatUTC)
	fileName := fmt.Sprintf("%s%sZ_%s_%s%s",
		procFilePrefix,
		timestamp,
		hex.EncodeToString([]byte(meta.DAGRunID)),
		hex.EncodeToString([]byte(meta.AttemptID)),
		procFileExt,
	)
	return filepath.Join(baseDir, meta.Name, fileName)
}

func parseProcFileName(filename string) (procFileName, error) {
	matches := procFileRegex.FindStringSubmatch(filename)
	if len(matches) != 4 {
		return procFileName{}, fmt.Errorf("%w: invalid proc filename %q", errInvalidProcFile, filename)
	}
	createdAt, err := time.Parse("20060102_150405Z", matches[1])
	if err != nil {
		return procFileName{}, fmt.Errorf("%w: parse proc timestamp: %w", errInvalidProcFile, err)
	}
	dagRunID, err := hex.DecodeString(matches[2])
	if err != nil {
		return procFileName{}, fmt.Errorf("%w: decode dag-run id: %w", errInvalidProcFile, err)
	}
	attemptID, err := hex.DecodeString(matches[3])
	if err != nil {
		return procFileName{}, fmt.Errorf("%w: decode attempt id: %w", errInvalidProcFile, err)
	}
	return procFileName{
		createdAt: createdAt.UTC(),
		dagRunID:  string(dagRunID),
		attemptID: string(attemptID),
	}, nil
}

func marshalProcMeta(meta exec.ProcMeta) ([]byte, error) {
	if err := validateProcMeta(meta); err != nil {
		return nil, err
	}
	return json.Marshal(procDiskMeta{
		Version:      procFileVersion,
		DAGName:      meta.Name,
		DAGRunID:     meta.DAGRunID,
		AttemptID:    meta.AttemptID,
		RootName:     meta.RootName,
		RootDAGRunID: meta.RootDAGRunID,
		StartedAt:    meta.StartedAt,
	})
}

func writeProcFile(fd *os.File, heartbeatUnix int64, meta exec.ProcMeta) error {
	metaBytes, err := marshalProcMeta(meta)
	if err != nil {
		return err
	}
	buf := make([]byte, heartbeatSize+len(metaBytes))
	binary.BigEndian.PutUint64(buf[:heartbeatSize], uint64(heartbeatUnix)) //nolint:gosec // heartbeat unix time is validated by caller
	copy(buf[heartbeatSize:], metaBytes)
	if err := fd.Truncate(0); err != nil {
		return err
	}
	if _, err := fd.WriteAt(buf, 0); err != nil {
		return err
	}
	return nil
}

func readProcEntry(path, groupName string, staleTime time.Duration, now time.Time) (exec.ProcEntry, error) {
	filename := filepath.Base(path)
	parsedName, err := parseProcFileName(filename)
	if err != nil {
		return exec.ProcEntry{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return exec.ProcEntry{}, err
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return exec.ProcEntry{}, err
	}
	if len(data) <= heartbeatSize {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc file %s is too short", errInvalidProcFile, path)
	}

	lastHeartbeatAt := int64(binary.BigEndian.Uint64(data[:heartbeatSize])) //nolint:gosec
	heartbeatTime := time.Unix(lastHeartbeatAt, 0).UTC()
	if heartbeatTime.After(now.Add(5 * time.Minute)) {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc heartbeat timestamp is in the future for %s", errInvalidProcFile, path)
	}

	var diskMeta procDiskMeta
	if err := json.Unmarshal(data[heartbeatSize:], &diskMeta); err != nil {
		return exec.ProcEntry{}, fmt.Errorf("%w: decode proc metadata: %w", errInvalidProcFile, err)
	}
	if diskMeta.Version != procFileVersion {
		return exec.ProcEntry{}, fmt.Errorf("%w: unsupported proc version %d", errInvalidProcFile, diskMeta.Version)
	}

	meta := exec.ProcMeta{
		StartedAt:    diskMeta.StartedAt,
		Name:         diskMeta.DAGName,
		DAGRunID:     diskMeta.DAGRunID,
		AttemptID:    diskMeta.AttemptID,
		RootName:     diskMeta.RootName,
		RootDAGRunID: diskMeta.RootDAGRunID,
	}
	if err := validateProcMeta(meta); err != nil {
		return exec.ProcEntry{}, fmt.Errorf("%w: %w", errInvalidProcFile, err)
	}

	if parsedName.dagRunID != meta.DAGRunID || parsedName.attemptID != meta.AttemptID {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc filename/body mismatch for %s", errInvalidProcFile, path)
	}
	if filepath.Base(filepath.Dir(path)) != meta.Name {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc path/body DAG name mismatch for %s", errInvalidProcFile, path)
	}

	fresh := now.Sub(info.ModTime()) < staleTime
	if !fresh {
		fresh = now.Sub(heartbeatTime) < staleTime
	}

	return exec.ProcEntry{
		GroupName:       groupName,
		FilePath:        path,
		Meta:            meta,
		LastHeartbeatAt: lastHeartbeatAt,
		Fresh:           fresh,
	}, nil
}

func sameProcEntry(a, b exec.ProcEntry) bool {
	return a.GroupName == b.GroupName &&
		a.FilePath == b.FilePath &&
		a.LastHeartbeatAt == b.LastHeartbeatAt &&
		a.Meta == b.Meta
}
