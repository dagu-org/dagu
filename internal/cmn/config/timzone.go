// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
// the IANA zone name (e.g. "Asia/Tokyo") or a POSIX fallback (e.g. "UTC-5") when unavailable.
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
	cfg.Location = time.Local
	_, cfg.TzOffsetInSec = time.Now().Zone()

	// Prefer the IANA zone name (e.g. "Asia/Tokyo") so that child
	// processes see a correct TZ value.  time.Local.String() returns the
	// IANA name on most platforms; fall back to a POSIX-style string only
	// when it is unavailable.  Note: POSIX TZ signs are inverted relative
	// to ISO 8601 (east-of-UTC is negative), so we negate the offset.
	name := time.Local.String()
	if name == "Local" || name == "" {
		if cfg.TzOffsetInSec != 0 {
			name = fmt.Sprintf("UTC%+d", -cfg.TzOffsetInSec/3600)
		} else {
			name = "UTC"
		}
	}
	cfg.TZ = name

	return nil
}
