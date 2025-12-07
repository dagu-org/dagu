package resource

import (
	"context"
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
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		config: cfg,
		store:  NewMemoryStore(cfg.Monitoring.Retention),
		done:   make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.loop(ctx)

	logger.Info(ctx, "Resource monitoring service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
	logger.Info(ctx, "Resource monitoring service stopped")
	return nil
}

func (s *Service) GetHistory(duration time.Duration) *ResourceHistory {
	return s.store.GetHistory(duration)
}

func (s *Service) loop(ctx context.Context) {
	defer close(s.done)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

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
	// CPU Usage
	cpuPercent, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		logger.Error(ctx, "Failed to get CPU usage", tag.Error(err))
		return
	}
	cpuVal := 0.0
	if len(cpuPercent) > 0 {
		cpuVal = cpuPercent[0]
	}

	// Memory Usage
	memStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to get memory usage", tag.Error(err))
		return
	}

	// Disk Usage (for data directory)
	diskStat, err := disk.UsageWithContext(ctx, s.config.Paths.DataDir)
	if err != nil {
		logger.Error(ctx, "Failed to get disk usage", tag.Error(err))
		return
	}

	// Load Average
	loadStat, err := load.AvgWithContext(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to get load average", tag.Error(err))
		return
	}

	s.store.Add(cpuVal, memStat.UsedPercent, diskStat.UsedPercent, loadStat.Load1)
}
