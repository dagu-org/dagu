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
	var tz string
	_, tzOffsetInSec := time.Now().Zone()

	if tzOffsetInSec != 0 {
		tz = fmt.Sprintf("UTC%+d", tzOffsetInSec/3600)
	} else {
		tz = "UTC"
	}

	cfg.Location = time.Local
	cfg.TZ = tz
	cfg.TzOffsetInSec = tzOffsetInSec

	return nil
}
