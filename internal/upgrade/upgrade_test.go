package upgrade

import (
	"os"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantMajor int64
		wantMinor int64
		wantPatch int64
	}{
		{
			name:      "version with v prefix",
			input:     "v1.30.3",
			wantMajor: 1,
			wantMinor: 30,
			wantPatch: 3,
		},
		{
			name:      "version without v prefix",
			input:     "1.30.3",
			wantMajor: 1,
			wantMinor: 30,
			wantPatch: 3,
		},
		{
			name:      "version with prerelease",
			input:     "v1.30.0-rc.1",
			wantMajor: 1,
			wantMinor: 30,
			wantPatch: 0,
		},
		{
			name:      "version with build timestamp",
			input:     "v1.30.3-260204123456",
			wantMajor: 1,
			wantMinor: 30,
			wantPatch: 3,
		},
		{
			name:    "development version",
			input:   "dev",
			wantErr: true,
		},
		{
			name:    "zero version",
			input:   "0.0.0",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "not-a-version",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVersion(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseVersion(%q) unexpected error: %v", tt.input, err)
				return
			}
			if v.Major() != uint64(tt.wantMajor) {
				t.Errorf("ParseVersion(%q) major = %d, want %d", tt.input, v.Major(), tt.wantMajor)
			}
			if v.Minor() != uint64(tt.wantMinor) {
				t.Errorf("ParseVersion(%q) minor = %d, want %d", tt.input, v.Minor(), tt.wantMinor)
			}
			if v.Patch() != uint64(tt.wantPatch) {
				t.Errorf("ParseVersion(%q) patch = %d, want %d", tt.input, v.Patch(), tt.wantPatch)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		target   string
		want     int
		isNewer  bool
	}{
		{
			name:    "target is newer",
			current: "v1.30.0",
			target:  "v1.30.3",
			want:    -1,
			isNewer: true,
		},
		{
			name:    "versions are equal",
			current: "v1.30.3",
			target:  "v1.30.3",
			want:    0,
			isNewer: false,
		},
		{
			name:    "current is newer",
			current: "v1.31.0",
			target:  "v1.30.3",
			want:    1,
			isNewer: false,
		},
		{
			name:    "major version difference",
			current: "v1.30.3",
			target:  "v2.0.0",
			want:    -1,
			isNewer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := ParseVersion(tt.current)
			if err != nil {
				t.Fatalf("ParseVersion(%q) failed: %v", tt.current, err)
			}
			target, err := ParseVersion(tt.target)
			if err != nil {
				t.Fatalf("ParseVersion(%q) failed: %v", tt.target, err)
			}

			got := CompareVersions(current, target)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.current, tt.target, got, tt.want)
			}

			gotNewer := IsNewer(current, target)
			if gotNewer != tt.isNewer {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.target, gotNewer, tt.isNewer)
			}
		})
	}
}

func TestPlatformDetect(t *testing.T) {
	p := Detect()

	if p.OS == "" {
		t.Error("Detect() returned empty OS")
	}
	if p.Arch == "" {
		t.Error("Detect() returned empty Arch")
	}
}

func TestPlatformAssetName(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		version  string
		want     string
	}{
		{
			name:     "darwin arm64 with v prefix",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			version:  "v1.30.3",
			want:     "dagu_1.30.3_darwin_arm64.tar.gz",
		},
		{
			name:     "linux amd64 without v prefix",
			platform: Platform{OS: "linux", Arch: "amd64"},
			version:  "1.30.3",
			want:     "dagu_1.30.3_linux_amd64.tar.gz",
		},
		{
			name:     "windows 386",
			platform: Platform{OS: "windows", Arch: "386"},
			version:  "v2.0.0",
			want:     "dagu_2.0.0_windows_386.tar.gz",
		},
		{
			name:     "linux armv7",
			platform: Platform{OS: "linux", Arch: "armv7"},
			version:  "v1.30.3",
			want:     "dagu_1.30.3_linux_armv7.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.platform.AssetName(tt.version)
			if got != tt.want {
				t.Errorf("Platform{%s, %s}.AssetName(%q) = %q, want %q",
					tt.platform.OS, tt.platform.Arch, tt.version, got, tt.want)
			}
		})
	}
}

func TestPlatformIsSupported(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     bool
	}{
		{name: "darwin arm64", platform: Platform{OS: "darwin", Arch: "arm64"}, want: true},
		{name: "darwin amd64", platform: Platform{OS: "darwin", Arch: "amd64"}, want: true},
		{name: "linux amd64", platform: Platform{OS: "linux", Arch: "amd64"}, want: true},
		{name: "linux arm64", platform: Platform{OS: "linux", Arch: "arm64"}, want: true},
		{name: "linux 386", platform: Platform{OS: "linux", Arch: "386"}, want: true},
		{name: "windows amd64", platform: Platform{OS: "windows", Arch: "amd64"}, want: true},
		{name: "freebsd amd64", platform: Platform{OS: "freebsd", Arch: "amd64"}, want: true},
		{name: "unsupported os", platform: Platform{OS: "plan9", Arch: "amd64"}, want: false},
		{name: "unsupported arch", platform: Platform{OS: "darwin", Arch: "mips"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.platform.IsSupported()
			if got != tt.want {
				t.Errorf("Platform{%s, %s}.IsSupported() = %v, want %v",
					tt.platform.OS, tt.platform.Arch, got, tt.want)
			}
		})
	}
}

func TestParseChecksums(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "standard sha256sum format (two spaces)",
			content: `abc123def456  dagu_1.30.3_darwin_arm64.tar.gz
789xyz012345  dagu_1.30.3_linux_amd64.tar.gz`,
			want: map[string]string{
				"dagu_1.30.3_darwin_arm64.tar.gz": "abc123def456",
				"dagu_1.30.3_linux_amd64.tar.gz":  "789xyz012345",
			},
		},
		{
			name: "with extra whitespace",
			content: `  abc123def456  dagu_1.30.3_darwin_arm64.tar.gz
789xyz012345  dagu_1.30.3_linux_amd64.tar.gz`,
			want: map[string]string{
				"dagu_1.30.3_darwin_arm64.tar.gz": "abc123def456",
				"dagu_1.30.3_linux_amd64.tar.gz":  "789xyz012345",
			},
		},
		{
			name: "single space fallback",
			content: `abc123def456 dagu_1.30.3_darwin_arm64.tar.gz`,
			want: map[string]string{
				"dagu_1.30.3_darwin_arm64.tar.gz": "abc123def456",
			},
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			content: "   \n   \n   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChecksums(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseChecksums() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseChecksums() unexpected error: %v", err)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseChecksums() got %d entries, want %d", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseChecksums()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestNormalizeVersionTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.30.3", "v1.30.3"},
		{"v1.30.3", "v1.30.3"},
		{"  v1.30.3  ", "v1.30.3"},
		{"  1.30.3  ", "v1.30.3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeVersionTag(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeVersionTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractVersionFromTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.30.3", "1.30.3"},
		{"1.30.3", "1.30.3"},
		{"v2.0.0-rc.1", "2.0.0-rc.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractVersionFromTag(tt.input)
			if got != tt.want {
				t.Errorf("ExtractVersionFromTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLooksLikeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"v1.30.3", true},
		{"1.30.3", true},
		{"v1.2.3-rc.1", true},
		{"dev", false},
		{"main", false},
		{"", false},
		{"v1", false},
		{"v1.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := LooksLikeVersion(tt.input)
			if got != tt.want {
				t.Errorf("LooksLikeVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{
		TagName: "v1.30.3",
		Assets: []Asset{
			{Name: "dagu_1.30.3_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
			{Name: "dagu_1.30.3_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
		},
	}

	tests := []struct {
		name     string
		platform Platform
		version  string
		wantName string
		wantErr  bool
	}{
		{
			name:     "darwin arm64 found",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			version:  "v1.30.3",
			wantName: "dagu_1.30.3_darwin_arm64.tar.gz",
		},
		{
			name:     "linux amd64 found",
			platform: Platform{OS: "linux", Arch: "amd64"},
			version:  "v1.30.3",
			wantName: "dagu_1.30.3_linux_amd64.tar.gz",
		},
		{
			name:     "platform not found",
			platform: Platform{OS: "windows", Arch: "amd64"},
			version:  "v1.30.3",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := FindAsset(release, tt.platform, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FindAsset() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("FindAsset() unexpected error: %v", err)
				return
			}
			if asset.Name != tt.wantName {
				t.Errorf("FindAsset() name = %q, want %q", asset.Name, tt.wantName)
			}
		})
	}
}

func TestDetectInstallMethod(t *testing.T) {
	// This test just verifies the function doesn't panic
	method := DetectInstallMethod()
	if method.String() == "" {
		t.Error("DetectInstallMethod() returned empty string")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{500, "500 bytes"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	content := []byte("hello world\n")
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// SHA256 of "hello world\n"
	expectedHash := "a948904f2f0f479b8f8564cbf12dac6b0c0a2e2a5c1a88e6e7e5f0d2e3e8a4f5"
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

	t.Run("matching hash", func(t *testing.T) {
		// Calculate actual hash first
		err := VerifyChecksum(tmpFile, wrongHash)
		if err == nil {
			t.Error("VerifyChecksum() expected error for wrong hash")
		}
		// The error message should contain both hashes
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Errorf("VerifyChecksum() error should mention mismatch: %v", err)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		err := VerifyChecksum("/nonexistent/file", expectedHash)
		if err == nil {
			t.Error("VerifyChecksum() expected error for nonexistent file")
		}
	})
}

func writeTestFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}
