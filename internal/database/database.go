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

	"github.com/yohamta/jobctl/internal/models"
	"github.com/yohamta/jobctl/internal/settings"
	"github.com/yohamta/jobctl/internal/utils"
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

func ParseFile(file string) (*models.StatusFile, error) {
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
	return &models.StatusFile{File: file, Status: m}, nil
}

func (db *Database) NewWriter(configPath string, t time.Time) (*Writer, string, error) {
	f, err := db.new(configPath, t)
	if err != nil {
		return nil, "", err
	}
	w := &Writer{
		filename: f,
	}
	return w, f, nil
}

func (db *Database) NewWriterFor(configPath string, file string) (*Writer, error) {
	if !utils.FileExists(file) {
		return nil, ErrNoDataFile
	}
	w := &Writer{
		filename: file,
	}
	return w, nil
}

func (db *Database) ReadStatusHist(configPath string, n int) ([]*models.StatusFile, error) {
	files, err := db.latest(configPath, n)
	if err != nil {
		return nil, err
	}
	ret := make([]*models.StatusFile, 0)
	for _, file := range files {
		status, err := ParseFile(file)
		if err != nil {
			continue
		}
		ret = append(ret, status)
	}
	return ret, nil
}

func (db *Database) ReadStatusToday(configPath string) (*models.Status, error) {
	file, err := db.latestToday(configPath, time.Now())
	if err != nil {
		return nil, err
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	l, err := findLastLine(f)
	if err != nil {
		return nil, err
	}
	m, err := models.StatusFromJson(l)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (db *Database) FindByRequestId(configPath string, requestId string) (*models.StatusFile, error) {
	pattern := db.pattern(configPath) + "*.dat"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w : %s", ErrNoDataFile, pattern)
	}
	sort.Slice(matches, func(i, j int) bool {
		return strings.Compare(matches[i], matches[j]) >= 0
	})
	for _, f := range matches {
		status, err := ParseFile(f)
		if err != nil {
			log.Printf("parsing failed %s : %s", f, err)
			continue
		}
		if status.Status != nil && status.Status.RequestId == requestId {
			return status, nil
		}
	}
	return nil, fmt.Errorf("%w : %s", ErrRequestIdNotFound, requestId)
}

func (db *Database) RemoveAll(configPath string) {
	db.RemoveOld(configPath, 0)
}

func (db *Database) RemoveOld(configPath string, retentionDays int) error {
	if retentionDays <= -1 {
		return nil
	}

	pattern := db.pattern(configPath) + "*.dat"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	ot := time.Now().AddDate(-1*retentionDays, 0, 0)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			log.Printf("%v", err)
			continue
		}
		if info.ModTime().Before(ot) {
			err := os.Remove(m)
			if err != nil {
				log.Printf("%v", err)
			}
		}
	}
	return err
}

func (db *Database) dir(configPath string, prefix string) string {
	h := md5.New()
	h.Write([]byte(configPath))
	v := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(db.Dir, fmt.Sprintf("%s-%s", prefix, v))
}

func (db *Database) new(configPath string, t time.Time) (string, error) {
	fileName := fmt.Sprintf("%s.%s.dat", db.pattern(configPath), t.Format("20060102.15:04:05"))
	if err := os.MkdirAll(path.Dir(fileName), 0755); err != nil {
		return "", err
	}
	return fileName, nil
}

func (db *Database) pattern(configPath string) string {
	p := prefix(configPath)
	dir := db.dir(configPath, p)
	return filepath.Join(dir, p)
}

func (db *Database) latestToday(configPath string, day time.Time) (string, error) {
	pattern := fmt.Sprintf("%s.%s*.dat", db.pattern(configPath), day.Format("20060102"))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	ret, err := filterLatest(matches, 1)
	if err != nil {
		return "", err
	}
	return ret[0], err
}

func (db *Database) latest(configPath string, n int) ([]string, error) {
	pattern := db.pattern(configPath) + "*.dat"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return []string{}, err
	}
	ret, err := filterLatest(matches, n)
	return ret, err
}

var (
	ErrNoDataFile        = fmt.Errorf("no data file found.")
	ErrRequestIdNotFound = fmt.Errorf("request id not found.")
)

var rTimestamp = regexp.MustCompile("2\\d{7}.\\d{2}.\\d{2}.\\d{2}")

func filterLatest(files []string, n int) ([]string, error) {
	if len(files) == 0 {
		return []string{}, ErrNoDataFile
	}
	sort.Slice(files, func(i, j int) bool {
		t1 := rTimestamp.FindString(files[i])
		t2 := rTimestamp.FindString(files[j])
		return t1 > t2
	})
	ret := make([]string, 0, n)
	for i := 0; i < n && i < len(files); i++ {
		ret = append(ret, files[i])
	}
	return ret, nil
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
		if err != nil {
			return "", err
		}
		ret = append(ret, b...)
		if !isPrefix {
			break
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
