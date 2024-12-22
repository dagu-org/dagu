// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package jsondb

import (
	"bufio"
	"context"

	// nolint: gosec
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

var (
	_ persistence.HistoryStore = (*JSONDB)(nil)

	errRequestIDNotFound  = errors.New("request ID not found")
	errCreateNewDirectory = errors.New("failed to create new directory")
	errKeyEmpty           = errors.New("dagFile is empty")

	// rTimestamp is a regular expression to match the timestamp in the file name.
	rTimestamp = regexp.MustCompile(`2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}|2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}Z`)
)

type Config struct {
	Location          string
	LatestStatusToday bool
	CacheSize         int
	CacheTTL          time.Duration
}

const (
	requestIDLenSafe  = 8
	extDat            = ".dat"
	dateTimeFormatUTC = "20060102.15:04:05.000Z"
	dateTimeFormat    = "20060102.15:04:05.000"
	dateFormat        = "20060102"
)

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		CacheSize:         300,
		CacheTTL:          3 * time.Hour,
		LatestStatusToday: true,
	}
}

// JSONDB manages DAGs status files in local storage.
type JSONDB struct {
	location string
	config   Config
	writer   *writer
	cache    *filecache.Cache[*model.Status]
}

// New creates a new JSONDB with default configuration.
func New(location string, cfg Config) *JSONDB {
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = DefaultConfig().CacheSize
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = DefaultConfig().CacheTTL
	}

	db := &JSONDB{
		config:   cfg,
		location: location,
		cache:    filecache.New[*model.Status](cfg.CacheSize, cfg.CacheTTL),
	}
	db.cache.StartEviction()
	return db
}

func (db *JSONDB) Update(ctx context.Context, key, requestID string, status *model.Status) error {
	statusFile, err := db.FindByRequestID(ctx, key, requestID)
	if err != nil {
		return err
	}

	writer := newWriter(statusFile.File)
	if err := writer.open(); err != nil {
		return err
	}

	defer func() {
		db.cache.Invalidate(statusFile.File)
		_ = writer.close()
	}()
	return writer.write(status)
}

func (db *JSONDB) Open(_ context.Context, key string, timestamp time.Time, requestID string) error {
	filePath, err := db.generateFilePath(key, newUTC(timestamp), requestID)
	if err != nil {
		return err
	}

	writer := newWriter(filePath)
	if err := writer.open(); err != nil {
		return err
	}

	db.writer = writer
	return nil
}

func (db *JSONDB) Write(_ context.Context, status *model.Status) error {
	return db.writer.write(status)
}

func (db *JSONDB) Close(ctx context.Context) error {
	if db.writer == nil {
		return nil
	}

	defer func() {
		_ = db.writer.close()
		db.writer = nil
	}()

	if err := db.Compact(ctx, db.writer.target); err != nil {
		return err
	}

	db.cache.Invalidate(db.writer.target)
	return db.writer.close()
}

func (db *JSONDB) ReadStatusRecent(_ context.Context, key string, itemLimit int) []*model.StatusFile {
	var ret []*model.StatusFile

	files := db.getLatestMatches(db.globPattern(key), itemLimit)
	for _, file := range files {
		status, err := db.cache.LoadLatest(file, func() (*model.Status, error) {
			return ParseStatusFile(file)
		})
		if err != nil {
			continue
		}
		ret = append(ret, &model.StatusFile{
			File:   file,
			Status: status,
		})
	}

	return ret
}

func (db *JSONDB) ReadStatusToday(_ context.Context, key string) (*model.Status, error) {
	file, err := db.latestToday(key, time.Now(), db.config.LatestStatusToday)
	if err != nil {
		return nil, err
	}

	return db.cache.LoadLatest(file, func() (*model.Status, error) {
		return ParseStatusFile(file)
	})
}

func (db *JSONDB) FindByRequestID(_ context.Context, key string, requestID string) (*model.StatusFile, error) {
	if requestID == "" {
		return nil, errRequestIDNotFound
	}

	matches, err := filepath.Glob(db.globPattern(key))
	if err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, match := range matches {
		status, err := ParseStatusFile(match)
		if err != nil {
			log.Printf("parsing failed %s : %s", match, err)
			continue
		}
		if status != nil && status.RequestID == requestID {
			return &model.StatusFile{
				File:   match,
				Status: status,
			}, nil
		}
	}

	return nil, fmt.Errorf("%w : %s", persistence.ErrRequestIDNotFound, requestID)
}

func (db *JSONDB) RemoveAll(ctx context.Context, key string) error {
	return db.RemoveOld(ctx, key, 0)
}

func (db *JSONDB) RemoveOld(_ context.Context, key string, retentionDays int) error {
	if retentionDays < 0 {
		return nil
	}

	matches, err := filepath.Glob(db.globPattern(key))
	if err != nil {
		return err
	}

	oldDate := time.Now().AddDate(0, 0, -retentionDays)
	var lastErr error
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().Before(oldDate) {
			if err := os.Remove(m); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

func (db *JSONDB) Compact(_ context.Context, targetFilePath string) error {
	status, err := ParseStatusFile(targetFilePath)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, targetFilePath)
	}

	newFile := fmt.Sprintf("%s_c.dat", strings.TrimSuffix(filepath.Base(targetFilePath), filepath.Ext(targetFilePath)))
	tempFilePath := filepath.Join(filepath.Dir(targetFilePath), newFile)
	writer := newWriter(tempFilePath)
	if err := writer.open(); err != nil {
		return err
	}
	defer writer.close()

	if err := writer.write(status); err != nil {
		if removeErr := os.Remove(tempFilePath); removeErr != nil {
			return fmt.Errorf("%w: %s", err, removeErr)
		}
		return fmt.Errorf("%w: %s", err, tempFilePath)
	}

	// remove the original file
	if err := os.Remove(targetFilePath); err != nil {
		return fmt.Errorf("%w: %s", err, targetFilePath)
	}

	// rename the file to the original
	if err := os.Rename(tempFilePath, targetFilePath); err != nil {
		return fmt.Errorf("%w: %s", err, targetFilePath)
	}

	return nil
}

func (db *JSONDB) Rename(_ context.Context, oldKey, newKey string) error {
	oldName := fileutil.EnsureYAMLExtension(oldKey)
	newName := fileutil.EnsureYAMLExtension(newKey)

	if !filepath.IsAbs(oldName) || !filepath.IsAbs(newName) {
		return fmt.Errorf("invalid path: %s -> %s", oldName, newName)
	}

	oldDir := db.getDirectory(oldName, getPrefix(oldName))
	if !db.exists(oldDir) {
		return nil
	}

	newDir := db.getDirectory(newName, getPrefix(newName))
	if !db.exists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", errCreateNewDirectory, newDir, err)
		}
	}

	matches, err := filepath.Glob(db.globPattern(oldName))
	if err != nil {
		return err
	}

	oldPrefix := filepath.Base(db.createPrefix(oldName))
	newPrefix := filepath.Base(db.createPrefix(newName))
	for _, m := range matches {
		base := filepath.Base(m)
		f := strings.Replace(base, oldPrefix, newPrefix, 1)
		if err := os.Rename(m, filepath.Join(newDir, f)); err != nil {
			log.Printf("failed to rename %s to %s: %s", m, f, err)
		}
	}
	if files, _ := os.ReadDir(oldDir); len(files) == 0 {
		_ = os.Remove(oldDir)
	}
	return nil
}

func (db *JSONDB) getDirectory(key string, prefix string) string {
	if key != prefix {
		// Add a hash postfix to the directory name to avoid conflicts.
		// nolint: gosec
		h := md5.New()
		_, _ = h.Write([]byte(key))
		v := hex.EncodeToString(h.Sum(nil))
		return filepath.Join(db.location, fmt.Sprintf("%s-%s", prefix, v))
	}

	return filepath.Join(db.location, key)
}

func (db *JSONDB) generateFilePath(key string, timestamp timeInUTC, requestID string) (string, error) {
	if key == "" {
		return "", errKeyEmpty
	}
	prefix := db.createPrefix(key)
	timestampString := timestamp.Format(dateTimeFormatUTC)
	requestID = util.TruncString(requestID, requestIDLenSafe)
	return fmt.Sprintf("%s.%s.%s.dat", prefix, timestampString, requestID), nil
}

func (db *JSONDB) latestToday(key string, day time.Time, latestStatusToday bool) (string, error) {
	prefix := db.createPrefix(key)
	pattern := fmt.Sprintf("%s.*.*.dat", prefix)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusDataToday
	}

	ret := filterLatest(matches, 1)
	if len(ret) == 0 {
		return "", persistence.ErrNoStatusData
	}

	startOfDay := day.Truncate(24 * time.Hour)
	startOfDayInUTC := newUTC(startOfDay)
	if latestStatusToday {
		timestamp, err := findTimestamp(ret[0])
		if err != nil {
			return "", err
		}
		if timestamp.Before(startOfDayInUTC.Time) {
			return "", persistence.ErrNoStatusDataToday
		}
	}

	return ret[0], nil
}

func (s *JSONDB) getLatestMatches(pattern string, itemLimit int) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	return filterLatest(matches, itemLimit)
}

func (s *JSONDB) globPattern(name string) string {
	return s.createPrefix(name) + "*" + extDat
}

func (s *JSONDB) createPrefix(name string) string {
	prefix := getPrefix(name)
	return filepath.Join(s.getDirectory(name, prefix), prefix)
}

func (s *JSONDB) exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func ParseStatusFile(filePath string) (*model.Status, error) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer f.Close()

	var (
		offset int64
		result *model.Status
	)
	for {
		line, err := readLineFrom(f, offset)
		if err == io.EOF {
			if result == nil {
				return nil, err
			}
			return result, nil
		} else if err != nil {
			return nil, err
		}
		offset += int64(len(line)) + 1 // +1 for newline
		if len(line) > 0 {
			status, err := model.StatusFromJSON(string(line))
			if err == nil {
				result = status
			}
		}
	}
}

func filterLatest(files []string, itemLimit int) []string {
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		a, err := findTimestamp(files[i])
		if err != nil {
			return false
		}
		b, err := findTimestamp(files[j])
		if err != nil {
			return true
		}
		return a.After(b)
	})
	return files[:min(len(files), itemLimit)]
}

func findTimestamp(file string) (time.Time, error) {
	timestampString := rTimestamp.FindString(file)
	if !strings.Contains(timestampString, "Z") {
		// For backward compatibility
		t, err := time.Parse(dateTimeFormat, timestampString)
		if err != nil {
			return time.Time{}, nil
		}
		return t, nil
	}

	// UTC
	t, err := time.Parse(dateTimeFormatUTC, timestampString)
	if err != nil {
		return time.Time{}, nil
	}
	return t, nil
}

func readLineFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(f)
	var ret []byte
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return ret, err
		}
		ret = append(ret, line...)
		if !isPrefix {
			break
		}
	}
	return ret, nil
}

func getPrefix(key string) string {
	ext := filepath.Ext(key)
	if ext == "" {
		// No extension
		return filepath.Base(key)
	}
	if fileutil.IsYAMLFile(key) {
		// Remove .yaml or .yml extension
		return strings.TrimSuffix(filepath.Base(key), ext)
	}
	// Use the base name (if it's a path or just a name)
	return filepath.Base(key)
}

// timeInUTC is a wrapper for time.Time that ensures the time is in UTC.
type timeInUTC struct{ time.Time }

func newUTC(t time.Time) timeInUTC {
	return timeInUTC{t.UTC()}
}
