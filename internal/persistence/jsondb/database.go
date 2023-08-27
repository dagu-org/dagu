package jsondb

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/yohamta/dagu/internal/persistence"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
)

// Store is the interfact to store workflow status in local.
// It stores status in JSON format in a directory as per each dagFile.
// Multiple JSON data can be stored in a single file and each data
// is separated by newline.
// When a data is updated, it appends a new line to the file.
// Only the latest data in a single file can be read.
// When Compact is called, it removes old data.
// Compact must be called only once per file.
type Store struct {
	*Config
	writer *writer
}

type Config struct {
	Dir string
}

// defaultConfig is the default configuration for Store.
func defaultConfig() *Config {
	return &Config{Dir: config.Get().DataDir}
}

// New creates a new Store with default configuration.
func New() *Store {
	return &Store{Config: defaultConfig()}
}

func (store *Store) Update(dagFile, requestId string, s *models.Status) error {
	f, err := store.FindByRequestId(dagFile, requestId)
	if err != nil {
		return err
	}
	w := &writer{target: f.File}
	if err := w.open(); err != nil {
		return err
	}
	defer func() {
		_ = w.close()
	}()
	return w.write(s)
}

func (store *Store) Open(dagFile string, t time.Time, requestId string) error {
	writer, _, err := store.newWriter(dagFile, t, requestId)
	if err != nil {
		return err
	}
	if err := writer.open(); err != nil {
		return err
	}
	store.writer = writer
	return nil
}

func (store *Store) Write(s *models.Status) error {
	return store.writer.write(s)
}

func (store *Store) Close() error {
	if store.writer == nil {
		return nil
	}
	defer func() {
		_ = store.writer.close()
	}()
	if err := store.Compact(store.writer.dagFile, store.writer.target); err != nil {
		return err
	}
	return store.writer.close()
}

// ParseFile parses a status file.
func ParseFile(file string) (*models.Status, error) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
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
func (store *Store) newWriter(dagFile string, t time.Time, requestId string) (*writer, string, error) {
	f, err := store.newFile(dagFile, t, requestId)
	if err != nil {
		return nil, "", err
	}
	w := &writer{target: f, dagFile: dagFile}
	return w, f, nil
}

// ReadStatusHist returns a list of status files.
func (store *Store) ReadStatusHist(dagFile string, n int) []*models.StatusFile {
	ret := make([]*models.StatusFile, 0)
	files := store.latest(store.pattern(dagFile)+"*.dat", n)
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
func (store *Store) ReadStatusToday(dagFile string) (*models.Status, error) {
	file, err := store.latestToday(dagFile, time.Now())
	if err != nil {
		return nil, err
	}
	return ParseFile(file)
}

// FindByRequestId finds a status file by requestId.
func (store *Store) FindByRequestId(dagFile string, requestId string) (*models.StatusFile, error) {
	if requestId == "" {
		return nil, fmt.Errorf("requestId is empty")
	}
	pattern := store.pattern(dagFile) + "*.dat"
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
	return nil, fmt.Errorf("%w : %s", persistence.ErrRequestIdNotFound, requestId)
}

// RemoveAll removes all files in a directory.
func (store *Store) RemoveAll(dagFile string) error {
	return store.RemoveOld(dagFile, 0)
}

// RemoveOld removes old files.
func (store *Store) RemoveOld(dagFile string, retentionDays int) error {
	pattern := store.pattern(dagFile) + "*.dat"
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
func (store *Store) Compact(_, original string) error {
	status, err := ParseFile(original)
	if err != nil {
		return err
	}

	newFile := fmt.Sprintf("%s_c.dat",
		strings.TrimSuffix(filepath.Base(original), path.Ext(original)))
	f := path.Join(filepath.Dir(original), newFile)
	w := &writer{target: f}
	if err := w.open(); err != nil {
		return err
	}
	defer func() {
		_ = w.close()
	}()

	if err := w.write(status); err != nil {
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
func (store *Store) Rename(oldPath, newPath string) error {
	oldDir := store.dir(oldPath, prefix(oldPath))
	newDir := store.dir(newPath, prefix(newPath))
	if !utils.FileExists(oldDir) {
		// No need to move data
		return nil
	}
	if !utils.FileExists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}
	matches, err := filepath.Glob(store.pattern(oldPath) + "*.dat")
	if err != nil {
		return err
	}
	oldPattern := path.Base(store.pattern(oldPath))
	newPattern := path.Base(store.pattern(newPath))
	for _, m := range matches {
		base := path.Base(m)
		f := strings.Replace(base, oldPattern, newPattern, 1)
		_ = os.Rename(m, path.Join(newDir, f))
	}
	if files, _ := os.ReadDir(oldDir); len(files) == 0 {
		_ = os.Remove(oldDir)
	}
	return nil
}

func (store *Store) dir(dagFile string, prefix string) string {
	h := md5.New()
	h.Write([]byte(dagFile))
	v := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(store.Dir, fmt.Sprintf("%s-%s", prefix, v))
}

func (store *Store) newFile(dagFile string, t time.Time, requestId string) (string, error) {
	if dagFile == "" {
		return "", fmt.Errorf("dagFile is empty")
	}
	fileName := fmt.Sprintf("%s.%s.%s.dat", store.pattern(dagFile), t.Format("20060102.15:04:05.000"), utils.TruncString(requestId, 8))
	return fileName, nil
}

func (store *Store) pattern(dagFile string) string {
	p := prefix(dagFile)
	dir := store.dir(dagFile, p)
	return filepath.Join(dir, p)
}

func (store *Store) latestToday(dagFile string, day time.Time) (string, error) {
	var ret []string
	pattern := fmt.Sprintf("%s.%s*.*.dat", store.pattern(dagFile), day.Format("20060102"))
	matches, err := filepath.Glob(pattern)
	if err == nil || len(matches) > 0 {
		ret = filterLatest(matches, 1)
	} else {
		return "", persistence.ErrNoStatusDataToday
	}
	if len(ret) == 0 {
		return "", persistence.ErrNoStatusData
	}
	return ret[0], err
}

func (store *Store) latest(pattern string, n int) []string {
	matches, err := filepath.Glob(pattern)
	var ret = []string{}
	if err == nil || len(matches) >= 0 {
		ret = filterLatest(matches, n)
	}
	return ret
}

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

func prefix(dagFile string) string {
	return strings.TrimSuffix(
		filepath.Base(dagFile),
		path.Ext(dagFile),
	)
}
