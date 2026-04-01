// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package workspace

import "testing"

func TestValidateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		valid    bool
		testName string
	}{
		{testName: "alphanumeric", name: "engineering1", valid: true},
		{testName: "underscore", name: "team_ai", valid: true},
		{testName: "empty", name: "", valid: false},
		{testName: "hyphen", name: "team-ai", valid: false},
		{testName: "space", name: "team ai", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.valid && err != nil {
				t.Fatalf("ValidateName(%q) returned error: %v", tt.name, err)
			}
			if !tt.valid && err != ErrInvalidWorkspaceName {
				t.Fatalf("ValidateName(%q) = %v, want %v", tt.name, err, ErrInvalidWorkspaceName)
			}
		})
	}
}
