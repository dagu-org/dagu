// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
package jsondb

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/history"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

var (
	_ history.Store = (*JSONDB)(nil)
)

const (
	defaultCacheSize     = 300
	safeRequestIDLen     = 8
	datePrefixFormat     = "20060102.15:04:05.000"
	dateFormat           = "20060102"
	statusDirRoot        = "status"
	statusFilename       = "status.jsonl"
	dagFilename          = "dag.yaml"
	indexFileExt         = ".dat"
	statusFileExt        = ".jsonl"
	compactedSuffix      = "_c"
	indexFileRegExpMatch = 5
	hashLength           = 5
)

var (
	// indexFileRegExp is the regexp for extracting the date and request ID from the status file name
	indexFileRegExp = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})\.\d{2}:\d{2}:\d{2}.\d{3}_([^.]+)\.dat`)
	// yearDirRegExp is used to match directories of the format YYYY, MM, or DD
	yearDirRegExp = regexp.MustCompile(`^\d{4}$`)
	// monthDirRegExp is used to match directories of the format MM
	monthDirRegExp = regexp.MustCompile(`^\d{2}$`)
	// dayDirRegExp is used to match directories of the format DD
	dayDirRegExp = regexp.MustCompile(`^\d{2}$`)
)

// safeName is a type for safe file names.
type safeName string

func newSafeName(dagID string) safeName {
	base := strings.TrimSuffix(filepath.Base(dagID), filepath.Ext(dagID))
	str := util.SafeName(base)
	hash := generateHash(str)
	// append the hash to the safe name
	return safeName(str + "_" + hash)
}

func (s safeName) String() string {
	return string(s)
}

// utcTime is a wrapper for time.Time that ensures the time is in UTC.
type utcTime struct {
	time.Time
}

func newUTC(t time.Time) utcTime {
	return utcTime{t.UTC()}
}

// JSONDB manages DAG status files in local storage.
type JSONDB struct {
	baseDir           string                          // Base directory for storing files
	writer            *writer                         // Current writer for active status updates
	cache             *filecache.Cache[*model.Status] // Cache for storing parsed status files
	latestStatusToday bool                            // Flag to determine if only today's latest status should be returned
	writerLock        sync.Mutex                      // Mutex for synchronizing access to shared resources
	logger            logger.Logger                   // Logger for recording events and errors
}

// New creates a new JSONDB instance with default configuration.
func New(baseDir string, logger logger.Logger, latestStatusToday bool) *JSONDB {
	s := &JSONDB{
		baseDir:           baseDir,
		cache:             filecache.New[*model.Status](defaultCacheSize, 3*time.Hour),
		latestStatusToday: latestStatusToday,
		logger:            logger,
	}
	s.cache.StartEviction()
	return s
}

// UpdateStatus updates the status of a specific DAG execution.
func (s *JSONDB) UpdateStatus(ctx context.Context, dagID, reqID string, status *model.Status) error {
	f, err := s.GetStatusByRequestID(ctx, dagID, reqID)
	if err != nil {
		return err
	}

	w, err := newWriter(f.File)
	if err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	defer func() {
		s.cache.Invalidate(f.File)
		_ = w.close()
	}()

	return w.write(status)
}

// Open initializes a new writer for a DAG execution.
func (s *JSONDB) Open(_ context.Context, dagID string, start time.Time, requestID string) error {
	if s.writer != nil {
		return history.ErrWriterOpen
	}

	s.writerLock.Lock()
	defer s.writerLock.Unlock()

	startTime := newUTC(start)

	// Status directory name format: <timestamp>_<requestID>
	// Status files are stored as <statusDirname>/status[_c].jsonl
	statusDirname := craftStatusDirname(startTime, requestID)

	// Create the index file for the DAG execution.
	// Index files will be used to efficiently retrieve status files for a given DAG.
	// Because the status files are stored in date-wise directories, the index file
	// have the format <statusDirname>.index.
	// We can construct the status file path from the index file path.
	indexFilename := filepath.Join(
		craftIndexDir(s.baseDir, newSafeName(dagID)),
		statusDirname+indexFileExt,
	)

	// make directories
	if err := os.MkdirAll(filepath.Dir(indexFilename), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// create index file if not exists
	if _, err := os.Stat(indexFilename); os.IsNotExist(err) {
		if _, err := os.Create(indexFilename); err != nil {
			return fmt.Errorf("failed to create index file: %w", err)
		}
	}

	// create status file
	writer, err := newWriter(
		filepath.Join(
			craftStatusDataDir(s.baseDir, startTime),
			statusDirname,
			statusFilename,
		),
	)

	if err != nil {
		return fmt.Errorf("failed to create status file: %w", err)
	}

	s.writer = writer

	return nil
}

// Write writes the current status to the active writer.
func (s *JSONDB) Write(_ context.Context, status *model.Status) error {
	s.writerLock.Lock()
	defer s.writerLock.Unlock()

	if s.writer == nil {
		return history.ErrWriterIsClosed
	}

	if err := s.writer.write(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}
	return nil
}

// Close finalizes the current writer and compacts the status file.
func (s *JSONDB) Close(_ context.Context) error {
	s.writerLock.Lock()

	if s.writer == nil {
		s.writerLock.Unlock()
		return history.ErrWriterIsClosed
	}

	defer func() {
		// invalidate cache
		s.cache.Invalidate(s.writer.statusFile)

		// close the file
		if s.writer == nil {
			return
		}
		if err := s.writer.close(); err != nil {
			s.logger.Errorf("failed to close file %s: %v", s.writer.statusFile, err)
		}

		s.writer = nil
		s.writerLock.Unlock()
	}()

	// compact the file
	if err := s.compact(s.writer.statusFile); err != nil {
		s.logger.Errorf("failed to compact file %s: %v", s.writer.statusFile, err)
	}

	return nil
}

// ListRecentStatuses retrieves the n most recent status files for a given DAG.
func (s *JSONDB) ListRecentStatuses(_ context.Context, dagID string, limit int) []*model.History {
	// Read the latest n status files for the given DAG.
	indexDir := craftIndexDir(s.baseDir, newSafeName(dagID))

	// If the index directory does not exist, return nil.
	if _, err := os.Stat(indexDir); os.IsNotExist(err) {
		return nil
	}

	// Search the index directory for the latest n status files.
	files, err := listFilesSorted(indexDir, indexFileExt, true)
	if err != nil {
		s.logger.Errorf("failed to list files in %s: %v", indexDir, err)
		return nil
	}
	files = files[:min(limit, len(files))]

	// Load the status of the latest n status files.
	var ret []*model.History
	for _, indexFile := range files {
		// Convert the index file to the status file.
		indexFileInfo, err := getIndexFileInfo(indexFile)
		if err != nil {
			s.logger.Errorf("failed to convert index file to status file: %v", err)
			continue
		}
		pattern := indexFileInfo.getStatusFilePattern(s.baseDir)

		// get the latest status file
		files, err := filepath.Glob(pattern)
		if err != nil {
			s.logger.Errorf("failed to list files in %s: %v", pattern, err)
			continue
		}

		// In most cases, there should be only one status file for a given index file.
		files = getLatestFiles(files, 1)
		if len(files) == 0 {
			s.logger.Errorf("no status files found for %s", indexFile)
			continue
		}
		statusFile := files[0]

		// Load the latest status file
		status, err := s.cache.LoadLatest(statusFile, func() (*model.Status, error) {
			return LoadStatusFile(statusFile)
		})
		if err != nil {
			s.logger.Errorf("failed to parse file %s: %v", indexFile, err)
			continue
		}

		ret = append(ret, &model.History{
			File:   statusFile,
			Status: status,
		})
	}

	return ret
}

// ListRecentAllDAGs retrieves the n most recent status files across all DAGs.
func (s *JSONDB) ListRecentStatusesAllDAGs(_ context.Context, n int) ([]*model.History, error) {
	// List recent files from the status directory
	recentFiles, err := s.listRecentFiles(filepath.Join(s.baseDir, statusDirRoot), n)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent files: %w", err)
	}

	var results []*model.History

	for _, file := range recentFiles {
		// Load the latest status file
		status, err := s.cache.LoadLatest(file, func() (*model.Status, error) {
			return LoadStatusFile(file)
		})
		if err != nil {
			s.logger.Errorf("failed to parse file %s: %v", file, err)
			continue
		}

		results = append(results, &model.History{
			File:   file,
			Status: status,
		})
	}

	// Sort results by file name
	sort.Slice(results, func(i, j int) bool {
		return strings.Compare(results[i].File, results[j].File) > 0
	})

	// Trim to the requested number of results
	if len(results) > n {
		results = results[:n]
	}

	return results, nil
}

// listRecentFiles lists the most recent n status files in reverse chronological order.
func (s *JSONDB) listRecentFiles(root string, limit int) ([]string, error) {
	var allFiles []string

	// Walk through the years in reverse order
	years, err := listDirsSorted(root, true, yearDirRegExp)
	if err != nil {
		return nil, fmt.Errorf("error listing years: %w", err)
	}

	for _, year := range years {
		yearPath := filepath.Join(root, year)

		// Walk through the months in reverse order
		months, err := listDirsSorted(yearPath, true, monthDirRegExp)
		if err != nil {
			return nil, fmt.Errorf("error listing months in %s: %w", year, err)
		}

		for _, month := range months {
			monthPath := filepath.Join(yearPath, month)

			// Walk through the days in reverse order
			days, err := listDirsSorted(monthPath, true, dayDirRegExp)
			if err != nil {
				return nil, fmt.Errorf("error listing days in %s/%s: %w", year, month, err)
			}

			for _, day := range days {
				dayPath := filepath.Join(monthPath, day)

				// List directories in the day directory in reverse order
				statusDirs, err := listDirsSorted(dayPath, true, nil)
				if err != nil {
					return nil, fmt.Errorf("error listing directories in %s/%s/%s: %w", year, month, day, err)
				}

				for _, statusDir := range statusDirs {
					// List files in the status directory
					file, err := getStatusFile(
						filepath.Join(dayPath, statusDir),
					)
					if err != nil {
						s.logger.Warn("failed to get status file: %s", err)
						continue
					}
					// only add the latest file for each status directory
					allFiles = append(allFiles, file)

					// If we have enough files, return them
					if len(allFiles) >= limit {
						return allFiles[:limit], nil
					}
				}
			}
		}
	}

	// If we don't have enough files, return all we found
	if len(allFiles) > limit {
		return allFiles[:limit], nil
	}
	return allFiles, nil
}

// GetLatest retrieves the latest status file for today for a given DAG.
func (s *JSONDB) GetLatestStatus(_ context.Context, dagID string) (*model.Status, error) {
	file, err := s.findLatestStatusFile(newSafeName(dagID), newUTC(time.Now()), s.latestStatusToday)
	if err != nil {
		return nil, err
	}

	return s.cache.LoadLatest(file, func() (*model.Status, error) {
		return LoadStatusFile(file)
	})
}

// GetByRequestID finds a status file by its request ID.
func (s *JSONDB) GetStatusByRequestID(_ context.Context, dagID string, reqID string) (*model.History, error) {
	if reqID == "" {
		return nil, fmt.Errorf("%w: requestID is empty", history.ErrReqIDNotFound)
	}
	indexDir := craftIndexDir(s.baseDir, newSafeName(dagID))
	safeReqID := safeRequestID(reqID)

	// Search for the index file for the given request ID
	pattern := filepath.Join(indexDir, "*"+safeReqID+"*"+indexFileExt)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// get the latest status file
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, f := range matches {
		indexFileInfo, err := getIndexFileInfo(f)
		if err != nil {
			s.logger.Warn("failed to convert index file to status file: %s", err)
			continue
		}
		pattern := indexFileInfo.getStatusFilePattern(s.baseDir)

		// get the latest status file
		statusFiles, err := filepath.Glob(pattern)
		if err != nil {
			s.logger.Warn("failed to list files in %s: %s", pattern, err)
			continue
		}

		for _, statusFile := range statusFiles {
			status, err := LoadStatusFile(statusFile)
			if err != nil {
				s.logger.Warn("parsing failed %s : %s", statusFile, err)
				continue
			}
			if status != nil && status.RequestID == reqID {
				return &model.History{File: statusFile, Status: status}, nil
			}
		}
	}

	return nil, fmt.Errorf("%w: %s", history.ErrReqIDNotFound, reqID)
}

// DeleteAll removes all status files for a given DAG.
func (s *JSONDB) DeleteAllStatuses(ctx context.Context, dagID string) error {
	return s.DeleteOldStatuses(ctx, dagID, 0)
}

// DeleteOld removes status files older than the specified retention period.
func (s *JSONDB) DeleteOldStatuses(_ context.Context, dagID string, retentionDays int) error {
	indexDir := craftIndexDir(s.baseDir, newSafeName(dagID))
	if retentionDays < 0 {
		return fmt.Errorf("retentionDays must be a non-negative integer: %d", retentionDays)
	}
	pattern := filepath.Join(indexDir, "*.dat")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	expiredTime := time.Now().AddDate(0, 0, -retentionDays)
	var lastErr error
	for _, m := range matches {
		indexFileInfo, err := getIndexFileInfo(m)
		if err != nil {
			s.logger.Warn("failed to convert index file to status file: %s", err)
			continue
		}
		pattern := indexFileInfo.getStatusFilePattern(s.baseDir)

		statusFiles, err := filepath.Glob(pattern)
		if err != nil {
			s.logger.Warn("failed to list files in %s: %s", pattern, err)
			continue
		}
		latestStatusFiles := getLatestFiles(statusFiles, 1)
		if len(latestStatusFiles) == 0 {
			s.logger.Warn("no status files found for %s", m)
			continue
		}
		info, err := os.Stat(latestStatusFiles[0])
		if err != nil {
			s.logger.Warn("failed to get file info %s: %s", latestStatusFiles[0], err)
			continue
		}
		if info.ModTime().After(expiredTime) {
			// skip if the file is not expired
			continue
		}
		// Remove the status file and the index file
		if err := os.Remove(m); err != nil {
			s.logger.Warn("failed to remove %s: %s", m, err)
			lastErr = err
		}
		for _, f := range statusFiles {
			if err := os.Remove(f); err != nil {
				s.logger.Warn("failed to remove %s: %s", f, err)
				lastErr = err
			}
		}
	}
	return lastErr
}

// Compact compresses the status file by keeping only the latest status.
func (s *JSONDB) compact(statusFile string) error {
	status, err := LoadStatusFile(statusFile)
	if err == io.EOF {
		// no data to compact
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", statusFile, err)
	}

	compactedFile, err := craftCompactedStatusFileName(statusFile)
	if err != nil {
		return err
	}

	w := &writer{statusFile: compactedFile}
	if err := w.open(); err != nil {
		return err
	}
	defer w.close()

	if err := w.write(status); err != nil {
		// rollback
		if removeErr := os.Remove(compactedFile); removeErr != nil {
			log.Printf("failed to remove %s : %s", compactedFile, removeErr)
		}
		return err
	}

	return os.Rename(compactedFile, statusFile)
}

// RenameDAG changes the ID of a DAG, effectively renaming its associated files.
func (s *JSONDB) RenameDAG(_ context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}

	oldIndexDir := craftIndexDir(s.baseDir, newSafeName(oldID))
	newIndexDir := craftIndexDir(s.baseDir, newSafeName(newID))

	if !pathExists(oldIndexDir) {
		// No index directory for the old DAG, nothing to rename
		return nil
	}

	// Check the new directory does not exist.
	// If it does, return an error.
	if pathExists(newIndexDir) {
		return fmt.Errorf("%w: %s", history.ErrConflict, newID)
	}

	// Rename the index directory.
	if err := os.Rename(oldIndexDir, newIndexDir); err != nil {
		return fmt.Errorf("failed to rename index directory: %w", err)
	}

	return nil
}

// findLatestStatusFile finds the latest status file for today or the most recent day.
func (s *JSONDB) findLatestStatusFile(dagID safeName, now utcTime, today bool) (string, error) {
	// Find the index file for the given DAG
	indexDir := craftIndexDir(s.baseDir, dagID)

	if today {
		start := now.Truncate(24 * time.Hour)
		end := start.Add(23 * time.Hour)

		for t := end; t.After(start) || t.Equal(start); t = t.Add(-time.Hour) {
			year, month, day := t.Date()
			hour := t.Hour()

			pattern := filepath.Join(indexDir,
				fmt.Sprintf("%04d%02d%02d.%02d*", year, month, day, hour),
			)
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) == 0 {
				continue
			}

			latestFiles := getLatestFiles(matches, 1)
			if len(latestFiles) == 0 {
				continue
			}

			status, err := s.indexFileToStatusFile(latestFiles[0])
			if err != nil {
				continue
			}
			return status, nil
		}

		return "", history.ErrNoStatusDataToday
	}

	matches, err := filepath.Glob(
		filepath.Join(indexDir, "*.dat"),
	)
	if err != nil || len(matches) == 0 {
		return "", history.ErrNoStatusDataToday
	}

	latestFiles := getLatestFiles(matches, 1)
	if len(latestFiles) == 0 {
		return "", history.ErrNoStatusData
	}

	return s.indexFileToStatusFile(latestFiles[0])
}

// indexFileToStatusFile converts an index file path to its corresponding status file path.
func (s *JSONDB) indexFileToStatusFile(indexFile string) (string, error) {
	indexFileInfo, err := getIndexFileInfo(indexFile)
	if err != nil {
		return "", err
	}
	pattern := indexFileInfo.getStatusFilePattern(s.baseDir)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	files = getLatestFiles(files, 1)
	if len(files) == 0 {
		return "", fmt.Errorf("no status files found for %s", indexFile)
	}
	return files[0], nil
}

// ListByLocalDate retrieves all status files for a specific date across all DAGs, using local timezone.
func (s *JSONDB) ListStatusesByDate(_ context.Context, date time.Time) ([]*model.History, error) {
	// Set the time to 00:00:00
	startOfDay := date.Truncate(24 * time.Hour)

	// Calculate the end of the day (exclusive)
	endOfDay := startOfDay.Add(24 * time.Hour)

	return s.listStatusInRange(newUTC(startOfDay), newUTC(endOfDay))
}

// listStatusInRange retrieves all status files for a specific date range.
// The range is inclusive of the start time and exclusive of the end time.
func (s *JSONDB) listStatusInRange(start, end utcTime) ([]*model.History, error) {
	var result []*model.History

	for t := start.Time; t.Before(end.Time); t = t.Add(time.Hour) {
		year, month, day := t.Date()
		hour := t.Hour()

		dayPath := filepath.Join(s.baseDir, statusDirRoot,
			fmt.Sprintf("%04d", year),
			fmt.Sprintf("%02d", month),
			fmt.Sprintf("%02d", day))

		pattern := filepath.Join(dayPath, fmt.Sprintf("%04d%02d%02d.%02d*", year, month, day, hour))
		statusDirs, err := filepath.Glob(pattern)
		if err != nil {
			s.logger.Errorf("failed to list status files for %s: %v", t.Format("2006-01-02 15:04"), err)
			continue
		}

		for _, statusDir := range statusDirs {
			// List files in the status directory
			file, err := getStatusFile(statusDir)
			if err != nil {
				s.logger.Warn("failed to get status file: %s", err)
				continue
			}

			status, err := s.cache.LoadLatest(file, func() (*model.Status, error) {
				return LoadStatusFile(file)
			})
			if err != nil {
				s.logger.Errorf("failed to parse file %s: %v", file, err)
				continue
			}

			result = append(result, &model.History{
				File:   file,
				Status: status,
			})
		}
	}

	// Sort status files by file name
	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i].File, result[j].File) > 0
	})

	return result, nil
}

func getStatusFile(statusDir string) (string, error) {
	files, err := listFilesSorted(statusDir, statusFileExt, true)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no status files found in %s", statusDir)
	}
	return files[0], nil
}

// pathExists checks if a given path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// LoadStatusFile reads and parses a status file, returning the latest status.
func LoadStatusFile(file string) (*model.Status, error) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer f.Close()

	var (
		offset int64
		ret    *model.Status
	)
	for {
		line, err := readLineFrom(f, offset)
		if err == io.EOF {
			if ret == nil {
				return nil, err
			}
			return ret, nil
		} else if err != nil {
			return nil, err
		}
		offset += int64(len(line)) + 1 // +1 for newline
		if len(line) > 0 {
			m, err := model.StatusFromJSON(string(line))
			if err == nil {
				ret = m
			}
		}
	}
}

// getLatestFiles returns the n most recent files from a given list.
func getLatestFiles(files []string, n int) []string {
	if len(files) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files[:min(n, len(files))]
}

// readLineFrom reads a line from a file starting at a specific offset.
func readLineFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	r := bufio.NewReader(f)
	var ret []byte
	for {
		b, isPrefix, err := r.ReadLine()
		if err != nil {
			return ret, err
		}
		ret = append(ret, b...)
		if !isPrefix {
			break
		}
	}
	return ret, nil
}

// craftCompactedStatusFileName creates a filename for a compacted status file.
func craftCompactedStatusFileName(file string) (string, error) {
	suffixWithExt := compactedSuffix + statusFileExt
	if strings.HasSuffix(file, suffixWithExt) {
		return "", history.ErrAlreadyCompacted
	}
	compactedFilename := strings.TrimSuffix(
		filepath.Base(file), filepath.Ext(file),
	) + suffixWithExt
	return filepath.Join(filepath.Dir(file), compactedFilename), nil
}

// craftIndexDir constructs the path to the index directory for a DAG.
func craftIndexDir(baseDir string, dagID safeName) string {
	return filepath.Join(baseDir, "index", dagID.String())
}

// craftStatusDataDir constructs the path to the status directory for a specific date.
func craftStatusDataDir(baseDir string, t utcTime) string {
	year := t.Format("2006")
	month := t.Format("01")
	date := t.Format("02")
	return filepath.Join(baseDir, statusDirRoot, year, month, date)
}

// craftStatusDirname generates a directory name for a status file.
func craftStatusDirname(t utcTime, requestID string) string {
	// status file name format: <timestamp>_<requestID>
	return fmt.Sprintf("%s_%s",
		t.Format(datePrefixFormat),
		safeRequestID(requestID),
	)
}

// indexFileInfo holds information parsed from an index file name.
type indexFileInfo struct {
	filePath string
	year     string
	month    string
	date     string
	reqID    string
}

// getIndexFileInfo extracts information from an index file name.
func getIndexFileInfo(indexFile string) (indexFileInfo, error) {
	base := filepath.Base(indexFile)
	m := indexFileRegExp.FindStringSubmatch(base)
	if len(m) != indexFileRegExpMatch {
		return indexFileInfo{}, fmt.Errorf("invalid index file: %s", indexFile)
	}
	return indexFileInfo{
		filePath: indexFile,
		year:     m[1],
		month:    m[2],
		date:     m[3],
		reqID:    m[4],
	}, nil
}

// StatusFilePattern returns a pattern to match status files for a given index file.
func (i indexFileInfo) getStatusFilePattern(baseDir string) string {
	statusDirname := strings.TrimSuffix(filepath.Base(i.filePath), filepath.Ext(i.filePath))
	return filepath.Join(
		baseDir,
		statusDirRoot,
		i.year, i.month, i.date,
		statusDirname,
		"*"+statusFileExt,
	)
}

// listDirsSorted lists directories in the given path, optionally in reverse order.
func listDirsSorted(path string, reverse bool, pattern *regexp.Regexp) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var dirs []string
	if pattern != nil {
		for _, entry := range entries {
			if entry.IsDir() && pattern.MatchString(entry.Name()) {
				dirs = append(dirs, entry.Name())
			}
		}
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}
	}

	if reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	} else {
		sort.Strings(dirs)
	}

	return dirs, nil
}

// listFilesSorted lists files in the given path, optionally in reverse order.
func listFilesSorted(path, ext string, reverse bool) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}

	if reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	} else {
		sort.Strings(files)
	}

	return files, nil
}

func safeRequestID(reqID string) string {
	return util.TruncString(reqID, safeRequestIDLen)
}

// generateHash creates a 5-character hash from the input string using SHA-256
func generateHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])[0:hashLength]
}
