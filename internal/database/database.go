package database

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

// Database is the interfact to store workflow status in local.
// It stores status in JSON format in a directory as per each configPath.
// Multiple JSON data can be stored in a single file and each data
// is separated by newline.
// When a data is updated, it appends a new line to the file.
// Only the latest data in a single file can be read.
// When Compact is called, it removes old data.
// Compact must be called only once per file.
type Database struct {
	*Config
}

type Config struct {
	Dir string
}

// DefaultConfig is the default configuration for Database.
func DefaultConfig() *Config {
	return &Config{
		Dir: settings.MustGet(settings.SETTING__DATA_DIR),
	}
}

// New creates a new Database with default configuration.
func New() *Database {
	return &Database{
		Config: DefaultConfig(),
	}
}

// ParseFile parses a status file.
func ParseFile(file string) (*models.Status, error) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer f.Close()
	var offset int64 = 0
	var ret *models.Status
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
			var m *models.Status
			m, err = models.StatusFromJson(string(line))
			if err == nil {
				ret = m
				continue
			}
		}
	}
}

// NewWriter creates a new writer for a status.
func (db *Database) NewWriter(configPath string, t time.Time, requestId string) (*Writer, string, error) {
	f, err := db.newFile(configPath, t, requestId)
	if err != nil {
		return nil, "", err
	}
	w := &Writer{Target: f}
	return w, f, nil
}

// ReadStatusHist returns a list of status files.
func (db *Database) ReadStatusHist(configPath string, n int) []*models.StatusFile {
	ret := make([]*models.StatusFile, 0)
	files := db.latest(db.pattern(configPath)+"*.dat", n)
	for _, file := range files {
		status, err := ParseFile(file)
		if err == nil {
			ret = append(ret, &models.StatusFile{
				File:   file,
				Status: status,
			})
		}
	}
	return ret
}

// ReadStatusToday returns a list of status files.
func (db *Database) ReadStatusToday(configPath string) (*models.Status, error) {
	file, err := db.latestToday(configPath, time.Now())
	if err != nil {
		return nil, err
	}
	return ParseFile(file)
}

// FindByRequestId finds a status file by requestId.
func (db *Database) FindByRequestId(configPath string, requestId string) (*models.StatusFile, error) {
	if requestId == "" {
		return nil, fmt.Errorf("requestId is empty")
	}
	pattern := db.pattern(configPath) + "*.dat"
	matches, err := filepath.Glob(pattern)
	if len(matches) > 0 || err == nil {
		sort.Slice(matches, func(i, j int) bool {
			return strings.Compare(matches[i], matches[j]) >= 0
		})
		for _, f := range matches {
			status, err := ParseFile(f)
			if err != nil {
				log.Printf("parsing failed %s : %s", f, err)
				continue
			}
			if status != nil && status.RequestId == requestId {
				return &models.StatusFile{
					File:   f,
					Status: status,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("%w : %s", ErrRequestIdNotFound, requestId)
}

// RemoveAll removes all files in a directory.
func (db *Database) RemoveAll(configPath string) error {
	return db.RemoveOld(configPath, 0)
}

// RemoveOld removes old files.
func (db *Database) RemoveOld(configPath string, retentionDays int) error {
	pattern := db.pattern(configPath) + "*.dat"
	var lastErr error = nil
	if retentionDays >= 0 {
		matches, _ := filepath.Glob(pattern)
		ot := time.Now().AddDate(0, 0, -1*retentionDays)
		for _, m := range matches {
			info, err := os.Stat(m)
			if err == nil {
				if info.ModTime().Before(ot) {
					lastErr = os.Remove(m)
				}
			}
		}
	}
	return lastErr
}

// Compact creates a new file with only the latest data and removes old data.
func (db *Database) Compact(configPath, original string) error {
	status, err := ParseFile(original)
	if err != nil {
		return err
	}

	new := fmt.Sprintf("%s_c.dat",
		strings.TrimSuffix(filepath.Base(original), path.Ext(original)))
	f := path.Join(filepath.Dir(original), new)
	w := &Writer{Target: f}
	if err := w.Open(); err != nil {
		return err
	}
	defer w.Close()

	if err := w.Write(status); err != nil {
		if err := os.Remove(f); err != nil {
			log.Printf("failed to remove %s : %s", f, err.Error())
		}
		return err
	}

	if err := os.Remove(original); err != nil {
		return err
	}

	return nil
}

// MoveData moves data from one directory to another.
func (db *Database) MoveData(oldPath, newPath string) error {
	oldDir := db.dir(oldPath, prefix(oldPath))
	newDir := db.dir(newPath, prefix(newPath))
	if !utils.FileExists(oldDir) {
		// No need to move data
		return nil
	}
	if !utils.FileExists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}
	matches, err := filepath.Glob(db.pattern(oldPath) + "*.dat")
	if err != nil {
		return err
	}
	oldPattern := path.Base(db.pattern(oldPath))
	newPattern := path.Base(db.pattern(newPath))
	for _, m := range matches {
		base := path.Base(m)
		f := strings.Replace(base, oldPattern, newPattern, 1)
		_ = os.Rename(m, path.Join(newDir, f))
	}
	if files, _ := os.ReadDir(oldDir); len(files) == 0 {
		os.Remove(oldDir)
	}
	return nil
}

func (db *Database) dir(configPath string, prefix string) string {
	h := md5.New()
	h.Write([]byte(configPath))
	v := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(db.Dir, fmt.Sprintf("%s-%s", prefix, v))
}

func (db *Database) newFile(configPath string, t time.Time, requestId string) (string, error) {
	if configPath == "" {
		return "", fmt.Errorf("configPath is empty")
	}
	fileName := fmt.Sprintf("%s.%s.%s.dat", db.pattern(configPath), t.Format("20060102.15:04:05.000"), utils.TruncString(requestId, 8))
	return fileName, nil
}

func (db *Database) pattern(configPath string) string {
	p := prefix(configPath)
	dir := db.dir(configPath, p)
	return filepath.Join(dir, p)
}

func (db *Database) latestToday(configPath string, day time.Time) (string, error) {
	var ret []string
	pattern := fmt.Sprintf("%s.%s*.*.dat", db.pattern(configPath), day.Format("20060102"))
	matches, err := filepath.Glob(pattern)
	if err == nil || len(matches) > 0 {
		ret = filterLatest(matches, 1)
	} else {
		return "", ErrNoStatusDataToday
	}
	if len(ret) == 0 {
		return "", ErrNoStatusData
	}
	return ret[0], err
}

func (db *Database) latest(pattern string, n int) []string {
	matches, err := filepath.Glob(pattern)
	var ret = []string{}
	if err == nil || len(matches) >= 0 {
		ret = filterLatest(matches, n)
	}
	return ret
}

var (
	ErrRequestIdNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

var rTimestamp = regexp.MustCompile(`2\d{7}.\d{2}:\d{2}:\d{2}`)

func filterLatest(files []string, n int) []string {
	if len(files) == 0 {
		return []string{}
	}
	sort.Slice(files, func(i, j int) bool {
		t1 := timestamp(files[i])
		t2 := timestamp(files[j])
		return t1 > t2
	})
	ret := make([]string, 0, n)
	for i := 0; i < n && i < len(files); i++ {
		ret = append(ret, files[i])
	}
	return ret
}

func timestamp(file string) string {
	return rTimestamp.FindString(file)
}

func readLineFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	r := bufio.NewReader(f)
	ret := []byte{}
	for {
		b, isPrefix, err := r.ReadLine()
		if err == io.EOF {
			return ret, err
		} else if err != nil {
			log.Printf("read line failed. %s", err)
			return nil, err
		}
		if err == nil {
			ret = append(ret, b...)
			if !isPrefix {
				break
			}
		}
	}
	return ret, nil
}

func prefix(configPath string) string {
	return strings.TrimSuffix(
		filepath.Base(configPath),
		path.Ext(configPath),
	)
}
