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

	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

var (
	_ persistence.HistoryStore = (*JSONDB)(nil)

	errRequestIDNotFound  = errors.New("request ID not found")
	errCreateNewDirectory = errors.New("failed to create new directory")
	errDAGFileEmpty       = errors.New("dagFile is empty")

	rTimestamp = regexp.MustCompile(`2\d{7}.\d{2}:\d{2}:\d{2}`)
)

const (
	defaultCacheSize = 300
	requestIDLenSafe = 8
	extDat           = ".dat"
	dateTimeFormat   = "20060102.15:04:05.000"
	dateFormat       = "20060102"
)

// JSONDB manages DAGs status files in local storage.
type JSONDB struct {
	location          string
	writer            *writer
	cache             *filecache.Cache[*model.Status]
	latestStatusToday bool
}

// New creates a new JSONDB with default configuration.
func New(location string, latestStatusToday bool) *JSONDB {
	s := &JSONDB{
		location:          location,
		cache:             filecache.New[*model.Status](defaultCacheSize, 3*time.Hour),
		latestStatusToday: latestStatusToday,
	}
	s.cache.StartEviction()
	return s
}

func (s *JSONDB) Update(dagFile, requestID string, status *model.Status) error {
	f, err := s.FindByRequestID(dagFile, requestID)
	if err != nil {
		return err
	}
	w := &writer{target: f.File}
	if err := w.open(); err != nil {
		return err
	}
	defer func() {
		s.cache.Invalidate(f.File)
		_ = w.close()
	}()
	return w.write(status)
}

func (s *JSONDB) Open(dagFile string, t time.Time, requestID string) error {
	writer, _, err := s.newWriter(dagFile, t, requestID)
	if err != nil {
		return err
	}
	if err := writer.open(); err != nil {
		return err
	}
	s.writer = writer
	return nil
}

func (s *JSONDB) Write(status *model.Status) error {
	return s.writer.write(status)
}

func (s *JSONDB) Close() error {
	if s.writer == nil {
		return nil
	}
	defer func() {
		_ = s.writer.close()
		s.writer = nil
	}()
	if err := s.Compact(s.writer.target); err != nil {
		return err
	}
	s.cache.Invalidate(s.writer.target)
	return s.writer.close()
}

func (s *JSONDB) newWriter(dagFile string, t time.Time, requestID string) (*writer, string, error) {
	f, err := s.newFile(dagFile, t, requestID)
	if err != nil {
		return nil, "", err
	}
	w := &writer{target: f, dagFile: dagFile}
	return w, f, nil
}

func (s *JSONDB) ReadStatusRecent(dagFile string, n int) []*model.StatusFile {
	var ret []*model.StatusFile
	files := s.latest(s.globPattern(dagFile), n)
	for _, file := range files {
		status, err := s.cache.LoadLatest(file, func() (*model.Status, error) {
			return ParseFile(file)
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

func (s *JSONDB) ReadStatusToday(dagFile string) (*model.Status, error) {
	file, err := s.latestToday(dagFile, time.Now(), s.latestStatusToday)
	if err != nil {
		return nil, err
	}
	return s.cache.LoadLatest(file, func() (*model.Status, error) {
		return ParseFile(file)
	})
}

func (s *JSONDB) FindByRequestID(dagFile string, requestID string) (*model.StatusFile, error) {
	if requestID == "" {
		return nil, errRequestIDNotFound
	}
	matches, err := filepath.Glob(s.globPattern(dagFile))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, f := range matches {
		status, err := ParseFile(f)
		if err != nil {
			log.Printf("parsing failed %s : %s", f, err)
			continue
		}
		if status != nil && status.RequestID == requestID {
			return &model.StatusFile{
				File:   f,
				Status: status,
			}, nil
		}
	}
	return nil, fmt.Errorf("%w : %s", persistence.ErrRequestIDNotFound, requestID)
} // FindByRequestId finds a status file by status.
func (s *JSONDB) FindByStatus(dagFile string) (*model.StatusFile, error) {
	matches, err := filepath.Glob(s.globPattern(dagFile))
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
			if status != nil && status.Status == scheduler.StatusQueue {
				return &model.StatusFile{
					File:   f,
					Status: status,
				}, nil
			}
		}
	}
	// return nil, fmt.Errorf("%w : %s", persistence.ErrRequestIdNotFound)
	return nil, nil
}

func (s *JSONDB) RemoveAll(dagFile string) error {
	return s.RemoveOld(dagFile, 0)
}

func (s *JSONDB) RemoveEmptyQueue(dagFile string) error {
	f, err := s.FindByStatus(dagFile)
	if f == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(f.File); err != nil {
		log.Printf("failed to remove %v : %s", f, err.Error())
		return err
	}
	return nil
}

func (s *JSONDB) RemoveOld(dagFile string, retentionDays int) error {
	if retentionDays < 0 {
		return nil
	}
	matches, err := filepath.Glob(s.globPattern(dagFile))
	if err != nil {
		return err
	}
	ot := time.Now().AddDate(0, 0, -retentionDays)
	var lastErr error
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().Before(ot) {
			if err := os.Remove(m); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (s *JSONDB) Compact(original string) error {
	status, err := ParseFile(original)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	newFile := fmt.Sprintf("%s_c.dat", strings.TrimSuffix(filepath.Base(original), filepath.Ext(original)))
	f := filepath.Join(filepath.Dir(original), newFile)
	w := &writer{target: f}
	if err := w.open(); err != nil {
		return err
	}
	defer w.close()

	if err := w.write(status); err != nil {
		if removeErr := os.Remove(f); removeErr != nil {
			log.Printf("failed to remove %s : %s", f, removeErr)
		}
		return err
	}

	return os.Remove(original)
}

func (s *JSONDB) Rename(oldID, newID string) error {
	on := util.AddYamlExtension(oldID)
	nn := util.AddYamlExtension(newID)

	if !filepath.IsAbs(on) || !filepath.IsAbs(nn) {
		return fmt.Errorf("invalid path: %s -> %s", on, nn)
	}

	oldDir := s.getDirectory(on, prefix(on))
	newDir := s.getDirectory(nn, prefix(nn))
	if !s.exists(oldDir) {
		return nil
	}
	if !s.exists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("%w: %s : %s", errCreateNewDirectory, newDir, err)
		}
	}
	matches, err := filepath.Glob(s.globPattern(on))
	if err != nil {
		return err
	}
	oldPrefix := filepath.Base(s.prefixWithDirectory(on))
	newPrefix := filepath.Base(s.prefixWithDirectory(nn))
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

func (s *JSONDB) getDirectory(name string, prefix string) string {
	// nolint: gosec
	h := md5.New()
	_, _ = h.Write([]byte(name))
	v := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(s.location, fmt.Sprintf("%s-%s", prefix, v))
}

func (s *JSONDB) newFile(dagFile string, t time.Time, requestID string) (string, error) {
	if dagFile == "" {
		return "", errDAGFileEmpty
	}
	return fmt.Sprintf(
		"%s.%s.%s.dat",
		s.prefixWithDirectory(dagFile),
		t.Format(dateTimeFormat),
		util.TruncString(requestID, requestIDLenSafe),
	), nil
}

func (s *JSONDB) latestToday(dagFile string, day time.Time, latestStatusToday bool) (string, error) {
	var pattern string
	if latestStatusToday {
		pattern = fmt.Sprintf("%s.%s*.*.dat", s.prefixWithDirectory(dagFile), day.Format(dateFormat))
	} else {
		pattern = fmt.Sprintf("%s.*.*.dat", s.prefixWithDirectory(dagFile))
	}
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", persistence.ErrNoStatusDataToday
	}
	ret := filterLatest(matches, 1)
	if len(ret) == 0 {
		return "", persistence.ErrNoStatusData
	}
	return ret[0], nil
}

func (s *JSONDB) latest(pattern string, n int) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	return filterLatest(matches, n)
}

func (s *JSONDB) globPattern(dagFile string) string {
	return s.prefixWithDirectory(dagFile) + "*" + extDat
}

func (s *JSONDB) prefixWithDirectory(dagFile string) string {
	p := prefix(dagFile)
	return filepath.Join(s.getDirectory(dagFile, p), p)
}

func (s *JSONDB) exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func ParseFile(file string) (*model.Status, error) {
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

func filterLatest(files []string, n int) []string {
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return timestamp(files[i]) > timestamp(files[j])
	})
	if n > len(files) {
		n = len(files)
	}
	return files[:n]
}

func timestamp(file string) string {
	return rTimestamp.FindString(file)
}

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

func prefix(dagFile string) string {
	return strings.TrimSuffix(filepath.Base(dagFile), filepath.Ext(dagFile))
}
