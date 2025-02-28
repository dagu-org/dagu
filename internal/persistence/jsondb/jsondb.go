package jsondb

import (
	"context"

	// nolint: gosec
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/dagu-org/dagu/internal/stringutil"
)

var (
	errRequestIDNotFound  = errors.New("request ID not found")
	errCreateNewDirectory = errors.New("failed to create new directory")
	errKeyEmpty           = errors.New("dagFile is empty")

	// rTimestamp is a regular expression to match the timestamp in the file name.
	rTimestamp = regexp.MustCompile(`2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}|2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}Z`)
)

type Config struct {
	Location          string
	LatestStatusToday bool
	FileCache         *filecache.Cache[*persistence.Status]
}

const (
	requestIDLenSafe  = 8
	extDat            = ".dat"
	dateTimeFormatUTC = "20060102.15:04:05.000Z"
	dateTimeFormat    = "20060102.15:04:05.000"
	dateFormat        = "20060102"
)

var _ persistence.HistoryStore = (*JSONDB)(nil)

// JSONDB manages DAGs status files in local storage.
type JSONDB struct {
	baseDir           string
	latestStatusToday bool
	cache             *filecache.Cache[*persistence.Status]
	writer            *writer
}

type Option func(*Options)

type Options struct {
	FileCache         *filecache.Cache[*persistence.Status]
	LatestStatusToday bool
}

func WithFileCache(cache *filecache.Cache[*persistence.Status]) Option {
	return func(o *Options) {
		o.FileCache = cache
	}
}

func WithLatestStatusToday(latestStatusToday bool) Option {
	return func(o *Options) {
		o.LatestStatusToday = latestStatusToday
	}
}

// New creates a new JSONDB instance.
func New(baseDir string, opts ...Option) *JSONDB {
	options := &Options{
		LatestStatusToday: true,
	}
	for _, opt := range opts {
		opt(options)
	}
	return &JSONDB{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
	}
}

func (db *JSONDB) Update(ctx context.Context, key, requestID string, status persistence.Status) error {
	historyRecord, err := db.FindByRequestID(ctx, key, requestID)
	if err != nil {
		return err
	}

	if err := historyRecord.Open(ctx); err != nil {
		return fmt.Errorf("failed to open history record: %w", err)
	}
	if err := historyRecord.Write(ctx, status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}
	if err := historyRecord.Close(ctx); err != nil {
		return fmt.Errorf("failed to close history record: %w", err)
	}
	return nil
}

func (db *JSONDB) NewStatus(ctx context.Context, key string, timestamp time.Time, requestID string) (persistence.HistoryRecord, error) {
	filePath, err := db.generateFilePath(key, newUTC(timestamp), requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file path: %w", err)
	}

	return NewHistoryRecord(filePath, db.cache), nil
}

func (db *JSONDB) ReadStatusRecent(_ context.Context, key string, itemLimit int) []persistence.HistoryRecord {
	var records []persistence.HistoryRecord

	files := db.getLatestMatches(db.globPattern(key), itemLimit)

	for _, file := range files {
		records = append(records, NewHistoryRecord(file, db.cache))
	}

	return records
}

func (db *JSONDB) ReadStatusToday(_ context.Context, key string) (persistence.HistoryRecord, error) {
	file, err := db.latestToday(key, time.Now(), db.latestStatusToday)
	if err != nil {
		return nil, fmt.Errorf("failed to read status today for %s: %w", key, err)
	}

	return NewHistoryRecord(file, db.cache), nil
}

func (db *JSONDB) FindByRequestID(_ context.Context, key string, requestID string) (persistence.HistoryRecord, error) {
	if requestID == "" {
		return nil, errRequestIDNotFound
	}

	matches, err := filepath.Glob(db.globPattern(key))
	if err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	for _, match := range matches {
		return NewHistoryRecord(match, db.cache), nil
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

func (db *JSONDB) Rename(_ context.Context, oldKey, newKey string) error {
	if !filepath.IsAbs(oldKey) || !filepath.IsAbs(newKey) {
		return fmt.Errorf("invalid path: %s -> %s", oldKey, newKey)
	}

	oldDir := db.getDirectory(oldKey, getPrefix(oldKey))
	if !db.exists(oldDir) {
		return nil
	}

	newDir := db.getDirectory(newKey, getPrefix(newKey))
	if !db.exists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", errCreateNewDirectory, newDir, err)
		}
	}

	matches, err := filepath.Glob(db.globPattern(oldKey))
	if err != nil {
		return err
	}

	oldPrefix := filepath.Base(db.createPrefix(oldKey))
	newPrefix := filepath.Base(db.createPrefix(newKey))
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
		return filepath.Join(db.baseDir, fmt.Sprintf("%s-%s", prefix, v))
	}

	return filepath.Join(db.baseDir, key)
}

func (db *JSONDB) generateFilePath(key string, timestamp timeInUTC, requestID string) (string, error) {
	if key == "" {
		return "", errKeyEmpty
	}
	prefix := db.createPrefix(key)
	timestampString := timestamp.Format(dateTimeFormatUTC)
	requestID = stringutil.TruncString(requestID, requestIDLenSafe)
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

func (s *JSONDB) globPattern(key string) string {
	return s.createPrefix(key) + "*" + extDat
}

func (s *JSONDB) createPrefix(key string) string {
	prefix := getPrefix(key)
	return filepath.Join(s.getDirectory(key, prefix), prefix)
}

func (s *JSONDB) exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
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
