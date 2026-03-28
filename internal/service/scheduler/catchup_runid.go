// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	// catchupPrefix is the scheme identifier for catchup run IDs.
	catchupPrefix = "catchup-"
	// oneOffPrefix is the scheme identifier for one-off scheduled run IDs.
	oneOffPrefix = "oneoff-"

	// maxRunIDLen matches exec.maxDAGRunIDLen (64 chars).
	maxRunIDLen = 64

	// hashLen is the number of hex characters from the SHA-256 hash.
	hashLen = 8

	// timestampLayout is the format for the scheduled time portion.
	timestampLayout = "20060102T150405"
)

// GenerateCatchupRunID produces a deterministic run ID for a catchup run.
// The format is: catchup-{sanitizedName}-{hash8}-{20060102T150405}
//
// The hash is derived from the original (unsanitized) DAG name, ensuring
// that DAGs whose names differ only in characters that get sanitized
// (e.g., dots vs underscores) still produce distinct run IDs.
//
// When the sanitized name exceeds the available space, it is truncated.
func GenerateCatchupRunID(dagName string, scheduledTime time.Time) string {
	return generateScheduledRunID(catchupPrefix, dagName, dagName, scheduledTime)
}

// GenerateOneOffRunID produces a deterministic run ID for a one-off schedule.
func GenerateOneOffRunID(dagName, fingerprint string, scheduledTime time.Time) string {
	return generateScheduledRunID(oneOffPrefix, dagName, dagName+":"+fingerprint, scheduledTime)
}

func generateScheduledRunID(prefix, dagName, hashSource string, scheduledTime time.Time) string {
	sanitized := sanitizeDagName(dagName)
	hash := dagNameHash(hashSource)
	ts := scheduledTime.UTC().Format(timestampLayout)

	maxNameLen := maxRunIDLen - len(prefix) - 1 - hashLen - 1 - len(timestampLayout)
	if len(sanitized) > maxNameLen {
		sanitized = sanitized[:maxNameLen]
	}

	return fmt.Sprintf("%s%s-%s-%s", prefix, sanitized, hash, ts)
}

// sanitizeDagName replaces characters not allowed in run IDs.
// Run IDs allow: [a-zA-Z0-9_-]. DAG names also allow dots,
// which are replaced with underscores to avoid ambiguity.
func sanitizeDagName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

// dagNameHash returns the first 8 hex characters of the SHA-256 hash
// of the original DAG name. This ensures uniqueness even when sanitization
// would collapse different names to the same string.
func dagNameHash(name string) string {
	h := sha256.Sum256([]byte(name))
	return hex.EncodeToString(h[:])[:hashLen]
}
