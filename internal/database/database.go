package database

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
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

type Database struct {
	*Config
}

type Config struct {
	Dir string
}

func New(config *Config) *Database {
	return &Database{
		Config: config,
	}
}

func DefaultConfig() *Config {
	return &Config{
		Dir: settings.MustGet(settings.CONFIG__DATA_DIR),
	}
}

func ParseFile(file string) (*models.Status, error) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer f.Close()
	l, err := findLastLine(f)
	if err != nil {
		log.Printf("failed to find last line. err: %v", err)
		return nil, err
	}
	m, err := models.StatusFromJson(l)
	if err != nil {
		log.Printf("failed to parse json. err: %v", err)
		return nil, err
	}
	return m, nil
}

func (db *Database) NewWriter(configPath string, t time.Time, requestId string) (*Writer, string, error) {
	f, err := db.newFile(configPath, t, requestId)
	if err != nil {
		return nil, "", err
	}
	w := &Writer{Target: f}
	return w, f, nil
}

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

func (db *Database) ReadStatusToday(configPath string) (*models.Status, error) {
	file, err := db.latestToday(configPath, time.Now())
	if err != nil {
		return nil, err
	}
	return ParseFile(file)
}

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

func (db *Database) RemoveAll(configPath string) {
	db.RemoveOld(db.pattern(configPath)+"*.dat", 0)
}

func (db *Database) RemoveOld(pattern string, retentionDays int) error {
	var lastErr error = nil
	if retentionDays >= 0 {
		matches, _ := filepath.Glob(pattern)
		ot := time.Now().AddDate(-1*retentionDays, 0, 0)
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

func (db *Database) MoveData(oldConfigPath, newConfigPath string) error {
	oldDir := db.dir(oldConfigPath, prefix(oldConfigPath))
	newDir := db.dir(newConfigPath, prefix(newConfigPath))
	if !utils.FileExists(oldDir) {
		// No need to move data
		return nil
	}
	if !utils.FileExists(newDir) {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}
	matches, err := filepath.Glob(db.pattern(oldConfigPath) + "*.dat")
	if err != nil {
		return err
	}
	oldPattern := path.Base(db.pattern(oldConfigPath))
	newPattern := path.Base(db.pattern(newConfigPath))
	for _, m := range matches {
		base := path.Base(m)
		f := strings.Replace(base, oldPattern, newPattern, 1)
		os.Rename(m, path.Join(newDir, f))
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
	var ret = []string{}
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

func findLastLine(f *os.File) (ret string, err error) {
	// seek to -2 position to the end of the file
	offset, err := f.Seek(-2, 2)
	if err != nil {
		return "", err
	}

	buf := make([]byte, 1)
	for {
		_, err = f.ReadAt(buf, offset)
		if err != nil {
			return "", err
		}
		// Find line break ('LF')
		// then read the line
		if buf[0] == byte('\n') {
			f.Seek(offset+1, 0)
			return readLineFrom(f)
		}
		// If offset == 0 then read the first line
		if offset == 0 {
			f.Seek(0, 0)
			str, err := readLineFrom(f)
			return str, err
		}
		offset--
	}
}

func readLineFrom(f *os.File) (string, error) {
	r := bufio.NewReader(f)
	ret := []byte{}
	for {
		b, isPrefix, err := r.ReadLine()
		utils.LogIgnoreErr("read line", err)
		if err == nil {
			ret = append(ret, b...)
			if !isPrefix {
				break
			}
		}
	}
	return string(ret), nil

}

func prefix(configPath string) string {
	return strings.TrimSuffix(
		filepath.Base(configPath),
		path.Ext(configPath),
	)
}
