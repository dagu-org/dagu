package fileutil

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
		{"Basic", "hello world", "hello_world"},
		{"Reserved characters", "file<>:\"/\\|!?*.txt", "file___________txt"},
		{"Reserved Windows names", "CON", "_con_"},
		{"Mixed case", "MixedCASE.txt", "mixedcase_txt"},
		{"Non-printable characters", "file\x00name.txt", "file_name_txt"},
		{"Leading and trailing spaces", " filename ", "_filename_"},
		{"Long filename", strings.Repeat("a", 150), strings.Repeat("a", 100)},
		{"All non-printable", "\x00\x01\x02", "___"},
		{"Unicode characters", "文件名.txt", "文件名_txt"},
		{"Empty string", "", ""},
		{"Dots and underscores", "...__", "_____"},
		{"Reserved Windows name with extension", "aux.txt", "aux_txt"},
		{"Multiple spaces", "multiple   spaces", "multiple___spaces"},
		{"Single period", "file.name", "file_name"},
		{"Multiple periods", "file...name", "file___name"},
		{"Leading period", ".hidden", "_hidden"},
		{"Trailing period", "visible.", "visible_"},
		{"Period and space", "file . name", "file___name"},
		{"Multiple periods and spaces", "file ...  name", "file______name"},
		{"Directory-like name", "my/directory/path", "my_directory_path"},
		{"File with multiple extensions", "script.tar.gz", "script_tar_gz"},
		{"Combination of issues", "My Weird File-Name!.txt", "my_weird_file-name__txt"},
		{"Multi-byte characters", "文件名" + strings.Repeat("あ", 100), "文件名" + strings.Repeat("あ", 92)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeName(tt.input)
			if !strings.HasPrefix(result, tt.expected) {
				t.Errorf("SafeName(%q) = %q, want prefix %q", tt.input, result, tt.expected)
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
			if reservedNamesRegex.MatchString(result) {
				t.Errorf("SafeName did not properly handle reserved Windows name %s: %s", name, result)
			}
		}
	})

	t.Run("Lowercase conversion", func(t *testing.T) {
		input := "MiXeDCaSe.TXT"
		result := SafeName(input)
		if result != strings.ToLower(result) {
			t.Errorf("SafeName did not convert to lowercase: %s", result)
		}
	})

	t.Run("No periods", func(t *testing.T) {
		inputs := []string{"file.name", "file..name", ".hidden", "visible.", "...", "a.b.c.d"}
		for _, input := range inputs {
			result := SafeName(input)
			if strings.Contains(result, ".") {
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
}
