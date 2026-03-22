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
	procFileTimeFmt   = dateTimeFormatUTC + "Z"
)

var (
	errInvalidProcFile  = errors.New("invalid proc file")
	procFileRegex       = regexp.MustCompile(`^proc_(\d{8}_\d{6}Z)_([0-9a-f]+)_([0-9a-f]+)\.proc$`)
	legacyProcFileRegex = regexp.MustCompile(`^proc_(\d{8}_\d{6}Z)_([-a-zA-Z0-9_]+)\.proc$`)
	reSafeID            = regexp.MustCompile(`^[-a-zA-Z0-9_]+$`)
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

type procFileFormat int

const (
	procFileFormatCurrent procFileFormat = iota + 1
	procFileFormatLegacy
)

type procFileName struct {
	format    procFileFormat
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
	if matches := procFileRegex.FindStringSubmatch(filename); len(matches) == 4 {
		createdAt, err := time.Parse(procFileTimeFmt, matches[1])
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
			format:    procFileFormatCurrent,
			createdAt: createdAt.UTC(),
			dagRunID:  string(dagRunID),
			attemptID: string(attemptID),
		}, nil
	}

	if matches := legacyProcFileRegex.FindStringSubmatch(filename); len(matches) == 3 {
		createdAt, err := time.Parse(procFileTimeFmt, matches[1])
		if err != nil {
			return procFileName{}, fmt.Errorf("%w: parse legacy proc timestamp: %w", errInvalidProcFile, err)
		}
		if err := exec.ValidateDAGRunID(matches[2]); err != nil {
			return procFileName{}, fmt.Errorf("%w: invalid legacy dag-run id: %w", errInvalidProcFile, err)
		}
		return procFileName{
			format:    procFileFormatLegacy,
			createdAt: createdAt.UTC(),
			dagRunID:  matches[2],
			attemptID: legacyProcAttemptID(matches[2]),
		}, nil
	}
	return procFileName{}, fmt.Errorf("%w: invalid proc filename %q", errInvalidProcFile, filename)
}

func legacyProcAttemptID(dagRunID string) string {
	return "legacy_" + hex.EncodeToString([]byte(dagRunID))
}

func legacyProcMeta(path string, parsedName procFileName, heartbeatTime time.Time, info os.FileInfo) (exec.ProcMeta, error) {
	dagName := filepath.Base(filepath.Dir(path))
	if dagName == "" || dagName == "." || dagName == string(filepath.Separator) {
		return exec.ProcMeta{}, fmt.Errorf("%w: invalid legacy proc path %s", errInvalidProcFile, path)
	}

	startedAt := parsedName.createdAt.UTC().Unix()
	if startedAt <= 0 {
		startedAt = heartbeatTime.UTC().Unix()
	}
	if startedAt <= 0 {
		startedAt = info.ModTime().UTC().Unix()
	}

	meta := exec.ProcMeta{
		StartedAt:    startedAt,
		Name:         dagName,
		DAGRunID:     parsedName.dagRunID,
		AttemptID:    parsedName.attemptID,
		RootName:     dagName,
		RootDAGRunID: parsedName.dagRunID,
	}
	if err := validateProcMeta(meta); err != nil {
		return exec.ProcMeta{}, fmt.Errorf("%w: %w", errInvalidProcFile, err)
	}
	return meta, nil
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
	if len(data) < heartbeatSize {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc file %s is shorter than the %d-byte heartbeat header", errInvalidProcFile, path, heartbeatSize)
	}

	lastHeartbeatAt := int64(binary.BigEndian.Uint64(data[:heartbeatSize])) //nolint:gosec
	heartbeatTime := time.Unix(lastHeartbeatAt, 0).UTC()
	if heartbeatTime.After(now.Add(5 * time.Minute)) {
		return exec.ProcEntry{}, fmt.Errorf("%w: proc heartbeat timestamp is in the future for %s", errInvalidProcFile, path)
	}

	var meta exec.ProcMeta
	switch parsedName.format {
	case procFileFormatCurrent:
		if len(data) == heartbeatSize {
			return exec.ProcEntry{}, fmt.Errorf("%w: proc file %s is missing metadata payload", errInvalidProcFile, path)
		}

		var diskMeta procDiskMeta
		if err := json.Unmarshal(data[heartbeatSize:], &diskMeta); err != nil {
			return exec.ProcEntry{}, fmt.Errorf("%w: decode proc metadata: %w", errInvalidProcFile, err)
		}
		if diskMeta.Version != procFileVersion {
			return exec.ProcEntry{}, fmt.Errorf("%w: unsupported proc version %d", errInvalidProcFile, diskMeta.Version)
		}

		meta = exec.ProcMeta{
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
	case procFileFormatLegacy:
		if len(data) != heartbeatSize {
			return exec.ProcEntry{}, fmt.Errorf("%w: legacy proc file %s must only contain the heartbeat header", errInvalidProcFile, path)
		}
		meta, err = legacyProcMeta(path, parsedName, heartbeatTime, info)
		if err != nil {
			return exec.ProcEntry{}, err
		}
	default:
		return exec.ProcEntry{}, fmt.Errorf("%w: unsupported proc filename format for %s", errInvalidProcFile, path)
	}

	// Use both filesystem mtime and the persisted heartbeat payload: recent writes
	// can make mtime fresher than the last synced payload, while the stored
	// heartbeat remains the durable fallback if mtime lags or is preserved.
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
