// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"testing"
)

func TestRole_Valid(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleOperator, true},
		{RoleViewer, true},
		{Role("invalid"), false},
		{Role(""), false},
		{Role("ADMIN"), false},  // case sensitive
		{Role("editor"), false}, // old role name
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.Valid(); got != tt.valid {
				t.Errorf("Role(%q).Valid() = %v, want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestRole_CanWrite(t *testing.T) {
	tests := []struct {
		role     Role
		canWrite bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleOperator, false},
		{RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.CanWrite(); got != tt.canWrite {
				t.Errorf("Role(%q).CanWrite() = %v, want %v", tt.role, got, tt.canWrite)
			}
		})
	}
}

func TestRole_CanExecute(t *testing.T) {
	tests := []struct {
		role       Role
		canExecute bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleOperator, true},
		{RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.CanExecute(); got != tt.canExecute {
				t.Errorf("Role(%q).CanExecute() = %v, want %v", tt.role, got, tt.canExecute)
			}
		})
	}
}

func TestRole_IsAdmin(t *testing.T) {
	tests := []struct {
		role    Role
		isAdmin bool
	}{
		{RoleAdmin, true},
		{RoleManager, false},
		{RoleOperator, false},
		{RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsAdmin(); got != tt.isAdmin {
				t.Errorf("Role(%q).IsAdmin() = %v, want %v", tt.role, got, tt.isAdmin)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input   string
		want    Role
		wantErr bool
	}{
		{"admin", RoleAdmin, false},
		{"manager", RoleManager, false},
		{"operator", RoleOperator, false},
		{"viewer", RoleViewer, false},
		{"invalid", "", true},
		{"", "", true},
		{"ADMIN", "", true},
		{"editor", "", true}, // old role name
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRole(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRole(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRole(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllRoles(t *testing.T) {
	roles := AllRoles()
	if len(roles) != 4 {
		t.Errorf("AllRoles() returned %d roles, want 4", len(roles))
	}

	// Ensure modifying returned slice doesn't affect internal state
	roles[0] = "modified"
	originalRoles := AllRoles()
	if originalRoles[0] == "modified" {
		t.Error("AllRoles() returned a reference to internal state")
	}
}
