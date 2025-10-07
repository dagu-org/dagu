package config

import (
	"fmt"
	"os"
	"time"
)

func setTimezone(cfg *Global) error {
	if cfg.TZ != "" {
		loc, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %w", err)
		}
		cfg.Location = loc
		_, cfg.TzOffsetInSec = time.Now().In(loc).Zone()

		if err := os.Setenv("TZ", cfg.TZ); err != nil {
			return fmt.Errorf("failed to set TZ environment variable: %w", err)
		}
		return nil
	}

	// Use local timezone when TZ is not specified
	cfg.Location = time.Local
	_, cfg.TzOffsetInSec = time.Now().Zone()

	if cfg.TzOffsetInSec == 0 {
		cfg.TZ = "UTC"
	} else {
		cfg.TZ = fmt.Sprintf("UTC%+d", cfg.TzOffsetInSec/3600)
	}

	return nil
}
