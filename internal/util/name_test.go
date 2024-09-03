// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package util

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSafeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Basic", "hello world", "hello_world_"},
		{"Reserved characters", "file<>:\"/\\|!?*.txt", "file___________txt_"},
		{"Reserved Windows names", "CON", "_con__"},
		{"Mixed case", "MixedCASE.txt", "mixedcase_txt_"},
		{"Non-printable characters", "file\x00name.txt", "file_name_txt_"},
		{"Leading and trailing spaces", " filename ", "_filename__"},
		{"Long filename", strings.Repeat("a", 150), strings.Repeat("a", 94) + "_"},
		{"All non-printable", "\x00\x01\x02", "____"},
		{"Unicode characters", "文件名.txt", "文件名_txt_"},
		{"Empty string", "", "_"},
		{"Dots and underscores", "...__", "_____"},
		{"Reserved Windows name with extension", "aux.txt", "aux_txt_"},
		{"Multiple spaces", "multiple   spaces", "multiple___spaces_"},
		{"Single period", "file.name", "file_name_"},
		{"Multiple periods", "file...name", "file___name_"},
		{"Leading period", ".hidden", "_hidden_"},
		{"Trailing period", "visible.", "visible__"},
		{"Period and space", "file . name", "file___name_"},
		{"Multiple periods and spaces", "file ...  name", "file______name_"},
		{"Directory-like name", "my/directory/path", "my_directory_path_"},
		{"File with multiple extensions", "script.tar.gz", "script_tar_gz_"},
		{"Combination of issues", "My Weird File-Name!.txt", "my_weird_file-name__txt_"},
		{"Multi-byte characters", "文件名" + strings.Repeat("あ", 100), "文件名" + strings.Repeat("あ", 91) + "_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeName(tt.input)
			if !strings.HasPrefix(result, tt.expected) {
				t.Errorf("SafeName(%q) = %q, want prefix %q", tt.input, result, tt.expected)
			}
			if !isValidHash(result[len(result)-5:]) {
				t.Errorf("SafeName(%q) did not append a valid 5-character hash: %q", tt.input, result[len(result)-5:])
			}
			if utf8.RuneCountInString(result) > 100 {
				t.Errorf("SafeName(%q) produced a result with more than 100 runes: %d", tt.input, utf8.RuneCountInString(result))
			}
		})
	}
}

func TestSafeNameProperties(t *testing.T) {
	t.Run("Length limit", func(t *testing.T) {
		longInput := strings.Repeat("a", 1000)
		result := SafeName(longInput)
		if utf8.RuneCountInString(result) != 100 {
			t.Errorf("SafeName produced a name with length other than 100 runes: %d", utf8.RuneCountInString(result))
		}
	})

	t.Run("No reserved characters", func(t *testing.T) {
		input := "test<>:\"/\\|!?*.file.txt"
		result := SafeName(input)
		if reservedCharRegex.MatchString(result[:len(result)-6]) {
			t.Errorf("SafeName produced a name with reserved characters: %s", result)
		}
	})

	t.Run("No reserved Windows names", func(t *testing.T) {
		reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "LPT1"}
		for _, name := range reservedNames {
			result := SafeName(name)
			if reservedNamesRegex.MatchString(result[:len(result)-6]) {
				t.Errorf("SafeName did not properly handle reserved Windows name %s: %s", name, result)
			}
		}
	})

	t.Run("Lowercase conversion", func(t *testing.T) {
		input := "MiXeDCaSe.TXT"
		result := SafeName(input)
		if result[:len(result)-6] != strings.ToLower(result[:len(result)-6]) {
			t.Errorf("SafeName did not convert to lowercase: %s", result)
		}
	})

	t.Run("No periods", func(t *testing.T) {
		inputs := []string{"file.name", "file..name", ".hidden", "visible.", "...", "a.b.c.d"}
		for _, input := range inputs {
			result := SafeName(input)
			if strings.Contains(result[:len(result)-6], ".") {
				t.Errorf("SafeName produced a name containing a period: %s", result)
			}
		}
	})

	t.Run("Uniqueness", func(t *testing.T) {
		input1 := "same_base_name"
		input2 := "same_base_name_but_longer"
		result1 := SafeName(input1)
		result2 := SafeName(input2)
		if result1 == result2 {
			t.Errorf("SafeName did not produce unique names for different inputs: %s and %s", result1, result2)
		}
	})

	t.Run("Hash consistency", func(t *testing.T) {
		input := "test_input"
		result1 := SafeName(input)
		result2 := SafeName(input)
		if result1 != result2 {
			t.Errorf("SafeName produced inconsistent results for the same input: %s and %s", result1, result2)
		}
	})
}

func isValidHash(s string) bool {
	if len(s) != 5 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
