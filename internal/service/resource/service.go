package resource

import (
	"context"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type Service struct {
	config *config.Config
	store  Store
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func NewService(cfg *config.Config) *Service {
	if cfg == nil {
		panic("config and config.Monitoring must not be nil")
	}
	return &Service{
		config: cfg,
		store:  NewMemoryStore(cfg.Monitoring.Retention),
		done:   make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil // Already started
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.loop(ctx)

	logger.Info(ctx, "Resource monitoring service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.mu.Unlock()
		<-s.done
	} else {
		s.mu.Unlock()
	}
	logger.Info(ctx, "Resource monitoring service stopped")
	return nil
}

func (s *Service) GetHistory(duration time.Duration) *ResourceHistory {
	return s.store.GetHistory(duration)
}

func (s *Service) loop(ctx context.Context) {
	defer close(s.done)

	ticker := time.NewTicker(s.config.Monitoring.Interval)
	defer ticker.Stop()

	// Collect initial metrics immediately
	s.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collect(ctx)
		}
	}
}

func (s *Service) collect(ctx context.Context) {
	var cpuVal, memVal, diskVal, loadVal float64

	// CPU Usage
	if cpuPercent, err := cpu.PercentWithContext(ctx, 100*time.Millisecond, false); err != nil {
		logger.Error(ctx, "Failed to get CPU usage", tag.Error(err))
	} else if len(cpuPercent) > 0 {
		cpuVal = cpuPercent[0]
	}

	// Memory Usage
	if memStat, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		logger.Error(ctx, "Failed to get memory usage", tag.Error(err))
	} else {
		memVal = memStat.UsedPercent
	}

	// Disk Usage (for data directory)
	if diskStat, err := disk.UsageWithContext(ctx, s.config.Paths.DataDir); err != nil {
		logger.Error(ctx, "Failed to get disk usage", tag.Error(err))
	} else {
		diskVal = diskStat.UsedPercent
	}

	// Load Average
	if loadStat, err := load.AvgWithContext(ctx); err != nil {
		logger.Error(ctx, "Failed to get load average", tag.Error(err))
	} else {
		loadVal = loadStat.Load1
	}

	s.store.Add(cpuVal, memVal, diskVal, loadVal)
}
