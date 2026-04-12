// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/core/exec"
)

const (
	queueIndexFileName = ".queue-index.json"
	queueIndexVersion  = 1
)

type queueReadIndex struct {
	Version  int      `json:"version"`
	Revision int64    `json:"revision"`
	High     []string `json:"high,omitempty"`
	Low      []string `json:"low,omitempty"`
}

type queueReadIndexCache struct {
	index   *queueReadIndex
	modTime time.Time
}

type queueReadCursor struct {
	Version     int    `json:"version"`
	Queue       string `json:"queue"`
	Revision    int64  `json:"revision"`
	Offset      int    `json:"offset"`
	AfterItemID string `json:"afterItemId"`
}

func newQueueReadIndex() *queueReadIndex {
	return &queueReadIndex{
		Version:  queueIndexVersion,
		Revision: time.Now().UTC().UnixNano(),
		High:     []string{},
		Low:      []string{},
	}
}

func (idx *queueReadIndex) ensureDefaults() {
	if idx.Version == 0 {
		idx.Version = queueIndexVersion
	}
	if idx.Revision == 0 {
		idx.Revision = time.Now().UTC().UnixNano()
	}
	if idx.High == nil {
		idx.High = []string{}
	}
	if idx.Low == nil {
		idx.Low = []string{}
	}
}

func (idx *queueReadIndex) total() int {
	return len(idx.High) + len(idx.Low)
}

func (idx *queueReadIndex) touch() {
	now := time.Now().UTC().UnixNano()
	if now <= idx.Revision {
		now = idx.Revision + 1
	}
	idx.Revision = now
}

func (idx *queueReadIndex) append(priority exec.QueuePriority, fileName string) {
	switch priority {
	case exec.QueuePriorityHigh:
		idx.High = append(idx.High, fileName)
	case exec.QueuePriorityLow:
		idx.Low = append(idx.Low, fileName)
	}
	idx.touch()
}

func (idx *queueReadIndex) removeItemID(itemID string) bool {
	if itemID == "" {
		return false
	}
	removed := removeQueueFileNameByID(&idx.High, itemID)
	if removeQueueFileNameByID(&idx.Low, itemID) {
		removed = true
	}
	if removed {
		idx.touch()
	}
	return removed
}

func (idx *queueReadIndex) itemFileNameAt(offset int) (string, bool) {
	if offset < 0 {
		return "", false
	}
	if offset < len(idx.High) {
		return idx.High[offset], true
	}
	offset -= len(idx.High)
	if offset < len(idx.Low) {
		return idx.Low[offset], true
	}
	return "", false
}

func (idx *queueReadIndex) resolveStart(cursor queueReadCursor) (int, error) {
	if cursor.Offset < 0 {
		return 0, exec.ErrInvalidCursor
	}
	if cursor.AfterItemID == "" {
		if cursor.Offset != 0 {
			return 0, exec.ErrInvalidCursor
		}
		return 0, nil
	}

	if cursor.Offset > 0 {
		if fileName, ok := idx.itemFileNameAt(cursor.Offset - 1); ok && queueItemIDFromFileName(fileName) == cursor.AfterItemID {
			return cursor.Offset, nil
		}
	}

	if offset := idx.findItemOffset(cursor.AfterItemID); offset >= 0 {
		return offset + 1, nil
	}

	return 0, exec.ErrInvalidCursor
}

func (idx *queueReadIndex) slice(start, limit int) []string {
	if start < 0 {
		start = 0
	}
	if limit <= 0 || start >= idx.total() {
		return nil
	}
	end := min(start+limit, idx.total())
	ret := make([]string, 0, end-start)
	for pos := start; pos < end; pos++ {
		fileName, ok := idx.itemFileNameAt(pos)
		if !ok {
			break
		}
		ret = append(ret, fileName)
	}
	return ret
}

func (idx *queueReadIndex) findItemOffset(itemID string) int {
	for pos, fileName := range idx.High {
		if queueItemIDFromFileName(fileName) == itemID {
			return pos
		}
	}
	for pos, fileName := range idx.Low {
		if queueItemIDFromFileName(fileName) == itemID {
			return len(idx.High) + pos
		}
	}
	return -1
}

func removeQueueFileNameByID(target *[]string, itemID string) bool {
	if len(*target) == 0 {
		return false
	}
	for i, fileName := range *target {
		if queueItemIDFromFileName(fileName) != itemID {
			continue
		}
		copy((*target)[i:], (*target)[i+1:])
		*target = (*target)[:len(*target)-1]
		return true
	}
	return false
}

func queueItemIDFromFileName(fileName string) string {
	base := filepath.Base(fileName)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func isHighQueueFileName(fileName string) bool {
	return strings.HasPrefix(fileName, "item_high_") && strings.HasSuffix(fileName, ".json")
}

func isLowQueueFileName(fileName string) bool {
	return strings.HasPrefix(fileName, "item_low_") && strings.HasSuffix(fileName, ".json")
}

func hasQueueItemEntries(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isHighQueueFileName(name) || isLowQueueFileName(name) {
			return true
		}
	}
	return false
}

func encodeQueueReadCursor(name string, idx *queueReadIndex, offset int, fileName string) string {
	if idx == nil || fileName == "" {
		return ""
	}
	return exec.EncodeSearchCursor(queueReadCursor{
		Version:     queueIndexVersion,
		Queue:       name,
		Revision:    idx.Revision,
		Offset:      offset,
		AfterItemID: queueItemIDFromFileName(fileName),
	})
}

func decodeQueueReadCursor(name, raw string) (queueReadCursor, error) {
	if raw == "" {
		return queueReadCursor{Version: queueIndexVersion, Queue: name}, nil
	}
	var cursor queueReadCursor
	if err := exec.DecodeSearchCursor(raw, &cursor); err != nil {
		return queueReadCursor{}, err
	}
	if cursor.Version != queueIndexVersion || cursor.Queue != name {
		return queueReadCursor{}, exec.ErrInvalidCursor
	}
	return cursor, nil
}

func (s *Store) queueDir(name string) string {
	return filepath.Join(s.baseDir, name)
}

func (s *Store) queueIndexPath(name string) string {
	return filepath.Join(s.queueDir(name), queueIndexFileName)
}

func (s *Store) loadOrRebuildQueueIndexLocked(ctx context.Context, name string) (*queueReadIndex, error) {
	queueDir := s.queueDir(name)
	if _, err := os.Stat(queueDir); os.IsNotExist(err) {
		delete(s.indices, name)
		return newQueueReadIndex(), nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat queue directory %s: %w", queueDir, err)
	}

	indexPath := s.queueIndexPath(name)
	info, statErr := os.Stat(indexPath)
	if statErr == nil {
		if cached, ok := s.indices[name]; ok && cached.index != nil && cached.modTime.Equal(info.ModTime()) {
			return cached.index, nil
		}
	}

	data, err := os.ReadFile(indexPath) //nolint:gosec
	switch {
	case err == nil:
		var idx queueReadIndex
		if jsonErr := json.Unmarshal(data, &idx); jsonErr == nil && idx.Version == queueIndexVersion {
			idx.ensureDefaults()
			modTime := time.Time{}
			if statErr == nil {
				modTime = info.ModTime()
			}
			s.indices[name] = &queueReadIndexCache{
				index:   &idx,
				modTime: modTime,
			}
			return &idx, nil
		}
		logger.Warn(ctx, "Queue index invalid, rebuilding",
			tag.Queue(name),
			tag.File(indexPath),
		)
	case os.IsNotExist(err):
		// Rebuild below.
	default:
		logger.Warn(ctx, "Failed to read queue index, rebuilding",
			tag.Queue(name),
			tag.File(indexPath),
			tag.Error(err),
		)
	}

	return s.rebuildQueueIndexLocked(ctx, name)
}

func (s *Store) rebuildQueueIndexLocked(ctx context.Context, name string) (*queueReadIndex, error) {
	queueDir := s.queueDir(name)
	idx := newQueueReadIndex()

	entries, err := os.ReadDir(queueDir)
	if os.IsNotExist(err) {
		return idx, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read queue directory %s: %w", queueDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch name := entry.Name(); {
		case isHighQueueFileName(name):
			idx.High = append(idx.High, name)
		case isLowQueueFileName(name):
			idx.Low = append(idx.Low, name)
		}
	}

	sort.Strings(idx.High)
	sort.Strings(idx.Low)

	if err := s.saveQueueIndexLocked(ctx, name, idx); err != nil {
		return nil, err
	}

	return idx, nil
}

func (s *Store) saveQueueIndexLocked(ctx context.Context, name string, idx *queueReadIndex) error {
	if idx == nil {
		return nil
	}

	queueDir := s.queueDir(name)
	indexPath := s.queueIndexPath(name)
	if idx.total() == 0 {
		delete(s.indices, name)
		if err := fileutil.RemoveWithRetry(indexPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove queue index %s: %w", indexPath, err)
		}
		return nil
	}

	idx.ensureDefaults()
	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("failed to marshal queue index %s: %w", name, err)
	}

	if err := os.MkdirAll(queueDir, 0750); err != nil { //nolint:gosec
		return fmt.Errorf("failed to ensure queue directory %s: %w", queueDir, err)
	}

	tmpFile, err := os.CreateTemp(queueDir, ".queue-index-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary queue index for %s: %w", name, err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write queue index for %s: %w", name, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary queue index for %s: %w", name, err)
	}
	if err := fileutil.RenameWithRetry(tmpFile.Name(), indexPath); err != nil {
		return fmt.Errorf("failed to install queue index for %s: %w", name, err)
	}

	modTime := time.Time{}
	if info, err := os.Stat(indexPath); err == nil {
		modTime = info.ModTime()
	}

	logger.Debug(ctx, "Queue index saved", tag.Queue(name), tag.Count(idx.total()))
	s.indices[name] = &queueReadIndexCache{
		index:   idx,
		modTime: modTime,
	}
	return nil
}

func (s *Store) invalidateQueueIndexLocked(ctx context.Context, name string) {
	delete(s.indices, name)
	indexPath := s.queueIndexPath(name)
	if err := fileutil.RemoveWithRetry(indexPath); err != nil && !os.IsNotExist(err) {
		logger.Warn(ctx, "Failed to invalidate queue index",
			tag.Queue(name),
			tag.File(indexPath),
			tag.Error(err),
		)
	}
}
