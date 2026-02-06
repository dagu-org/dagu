package cmdutil

import "testing"

func TestHasShellArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  bool
	}{
		{name: "NilSlice", input: nil, want: false},
		{name: "EmptySlice", input: []string{}, want: false},
		{name: "OnlyWhitespace", input: []string{"", " "}, want: false},
		{name: "DirectOnly", input: []string{"direct"}, want: false},
		{name: "DirectWithWhitespace", input: []string{" direct "}, want: false},
		{name: "DirectWithArgsIgnored", input: []string{"direct", "-c"}, want: false},
		{name: "SingleShell", input: []string{"/bin/sh"}, want: true},
		{name: "ShellWithArgs", input: []string{"/bin/sh", "-c"}, want: true},
		{name: "ShellAfterWhitespace", input: []string{" ", "/bin/bash"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := HasShellArgs(tt.input); got != tt.want {
				t.Fatalf("HasShellArgs(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsShellValueSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{name: "Nil", input: nil, want: false},
		{name: "EmptyString", input: "", want: false},
		{name: "WhitespaceString", input: "  ", want: false},
		{name: "DirectString", input: "direct", want: false},
		{name: "DirectStringTrim", input: " direct ", want: false},
		{name: "ShellString", input: "/bin/sh", want: true},
		{name: "ShellArgsSlice", input: []string{"/bin/sh", "-c"}, want: true},
		{name: "DirectSlice", input: []string{"direct"}, want: false},
		{name: "DirectSliceWithArgs", input: []string{"direct", "-c"}, want: false},
		{name: "AnySliceShell", input: []any{"/bin/sh", "-c"}, want: true},
		{name: "AnySliceDirect", input: []any{"direct"}, want: false},
		{name: "AnySliceDirectWithArgs", input: []any{"direct", "-c"}, want: false},
		{name: "AnySliceNonString", input: []any{123}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsShellValueSet(tt.input); got != tt.want {
				t.Fatalf("IsShellValueSet(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
