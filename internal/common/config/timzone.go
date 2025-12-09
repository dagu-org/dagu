package config

import (
	"fmt"
	"os"
	"time"
)

// setTimezone configures the timezone fields of cfg based on cfg.TZ or the local system timezone.
//
// If cfg.TZ is non-empty, it loads the corresponding time.Location, assigns it to cfg.Location,
// updates cfg.TzOffsetInSec with the current offset for that location, and sets the process
// TZ environment variable to cfg.TZ. It returns an error if loading the location or setting
// the environment variable fails.
//
// If cfg.TZ is empty, it uses the system local timezone: cfg.Location is set to time.Local,
// cfg.TzOffsetInSec is set to the current local offset in seconds, and cfg.TZ is populated with
// "UTC" or "UTC±H" where H is the offset in hours (e.g., "UTC+2" or "UTC-5").
//
// Returns an error only when loading a specified timezone or setting the TZ environment variable fails.
func setTimezone(cfg *Core) error {
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
