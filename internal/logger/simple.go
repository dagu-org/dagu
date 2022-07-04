package logger

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/yohamta/dagu/internal/utils"
)

type SimpleLogger struct {
	Dir     string
	LogName string
	Period  time.Duration
	mu      sync.Mutex
	file    *os.File
	stop    chan struct{}
}

var _ io.Writer = (*SimpleLogger)(nil)

func NewSimpleLogger(dir, logName string, period time.Duration) *SimpleLogger {
	return &SimpleLogger{
		Dir:     dir,
		LogName: logName,
		Period:  period,
		stop:    make(chan struct{}),
	}
}

func (rl *SimpleLogger) Open() error {
	err := rl.setupFile()
	if err != nil {
		return err
	}
	go func() {
		timer := time.NewTimer(time.Until(rl.timeToSwitchLog()))
		for {
			select {
			case <-timer.C:
				rl.mu.Lock()
				err := rl.setupFile()
				utils.LogErr("setup log file", err)
				timer = time.NewTimer(time.Until(rl.timeToSwitchLog()))
				rl.mu.Unlock()
			case <-rl.stop:
				timer.Stop()
				return
			}
		}
	}()
	return nil
}

func (rl *SimpleLogger) Write(p []byte) (n int, err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.file != nil {
		return rl.file.Write(p)
	}
	return 0, nil
}

func (rl *SimpleLogger) Close() (err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	err = rl.closeFile()
	rl.stop <- struct{}{}
	return nil
}

func (rl *SimpleLogger) setupFile() (err error) {
	rl.closeFile()
	filename := path.Join(
		rl.Dir,
		fmt.Sprintf("%s%s.log",
			rl.LogName,
			time.Now().Format("20060102.15:04:05.000"),
		))
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	rl.file, err = utils.OpenOrCreateFile(filename)
	return
}

func (rl *SimpleLogger) closeFile() (err error) {
	if rl.file != nil {
		_ = rl.file.Sync()
		err = rl.file.Close()
	}
	return
}

func (rl *SimpleLogger) timeToSwitchLog() time.Time {
	return time.Now().Add(rl.Period).Truncate(rl.Period)
}
