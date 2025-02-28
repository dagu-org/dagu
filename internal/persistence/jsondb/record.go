package jsondb

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

type HistoryRecord struct {
	file   string
	writer *writer
	mu     sync.Mutex
	cache  *filecache.Cache[*persistence.Status]
}

func NewHistoryRecord(file string, cache *filecache.Cache[*persistence.Status]) *HistoryRecord {
	return &HistoryRecord{
		file:  file,
		cache: cache,
	}
}

func (hr *HistoryRecord) Open(ctx context.Context) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if hr.writer != nil {
		return fmt.Errorf("status file already open: %w", ErrStatusFileOpen)
	}

	logger.Infof(ctx, "Initializing status file: %s", hr.file)

	writer := newWriter(hr.file)
	if err := writer.open(); err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	hr.writer = writer
	return nil
}

func (hr *HistoryRecord) Write(_ context.Context, status persistence.Status) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	if hr.writer == nil {
		return fmt.Errorf("status file not open: %w", ErrStatusFileNotOpen)
	}

	return hr.writer.write(status)
}

func (hr *HistoryRecord) Close(ctx context.Context) error {
	if hr.writer == nil {
		return nil
	}

	defer func() {
		_ = hr.writer.close()
		hr.writer = nil
	}()

	if err := hr.Compact(ctx); err != nil {
		return err
	}

	if hr.cache != nil {
		hr.cache.Invalidate(hr.file)
	}
	return hr.writer.close()
}

func (hr *HistoryRecord) Compact(_ context.Context) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	status, err := hr.parse()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, hr.file)
	}

	// Create a new file with compacted data
	newFile := fmt.Sprintf("%s_c.dat", strings.TrimSuffix(filepath.Base(hr.file), filepath.Ext(hr.file)))
	tempFilePath := filepath.Join(filepath.Dir(hr.file), newFile)
	writer := newWriter(tempFilePath)
	if err := writer.open(); err != nil {
		return err
	}
	defer writer.close()

	if err := writer.write(*status); err != nil {
		if removeErr := os.Remove(tempFilePath); removeErr != nil {
			return fmt.Errorf("%w: %s", err, removeErr)
		}
		return fmt.Errorf("%w: %s", err, tempFilePath)
	}

	// Remove old file and rename temp file
	if err := os.Remove(hr.file); err != nil {
		return fmt.Errorf("%w: %s", err, hr.file)
	}

	if err := os.Rename(tempFilePath, hr.file); err != nil {
		return fmt.Errorf("%w: %s", err, hr.file)
	}

	return nil
}

func (hr *HistoryRecord) ReadStatus() (*persistence.Status, error) {
	statusFile, err := hr.Read()
	if err != nil {
		return nil, err
	}
	return &statusFile.Status, nil
}

func (hr *HistoryRecord) Read() (*persistence.StatusFile, error) {
	if hr.cache != nil {
		status, err := hr.cache.LoadLatest(hr.file, func() (*persistence.Status, error) {
			return hr.parse()
		})
		if err == nil {
			return persistence.NewStatusFile(hr.file, *status), nil
		}
	}
	parsed, err := hr.parse()
	if err != nil {
		return nil, err
	}
	return persistence.NewStatusFile(hr.file, *parsed), nil
}

func (hr *HistoryRecord) parse() (*persistence.Status, error) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	f, err := os.Open(hr.file)
	if err != nil {
		log.Printf("failed to open file. err: %v", err)
		return nil, err
	}
	defer f.Close()

	var (
		offset int64
		result *persistence.Status
	)

	// Read append-only file from the end and find the last status
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
			status, err := persistence.StatusFromJSON(string(line))
			if err == nil {
				result = status
			}
		}
	}
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
