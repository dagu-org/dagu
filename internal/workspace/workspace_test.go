// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package workspace

import (
	"errors"
	"testing"
)

func TestValidateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		valid    bool
		testName string
	}{
		{testName: "alphanumeric", name: "engineering1", valid: true},
		{testName: "underscore", name: "team_ai", valid: true},
		{testName: "hyphen", name: "team-ai", valid: true},
		{testName: "empty", name: "", valid: false},
		{testName: "space", name: "team ai", valid: false},
		{testName: "reserved all", name: "all", valid: false},
		{testName: "reserved default", name: "default", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.valid && err != nil {
				t.Fatalf("ValidateName(%q) returned error: %v", tt.name, err)
			}
			if !tt.valid && !errors.Is(err, ErrInvalidWorkspaceName) {
				t.Fatalf("ValidateName(%q) = %v, want %v", tt.name, err, ErrInvalidWorkspaceName)
			}
		})
	}
}
