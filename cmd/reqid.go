// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import "github.com/google/uuid"

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
