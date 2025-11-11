package pathutil

import "testing"

func TestBuildPublicEndpointPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		suffix   string
		want     string
	}{
		{
			name:     "both empty",
			basePath: "",
			suffix:   "",
			want:     "",
		},
		{
			name:     "empty base",
			basePath: "",
			suffix:   "api/v2/health",
			want:     "/api/v2/health",
		},
		{
			name:     "empty suffix",
			basePath: "/base",
			suffix:   "",
			want:     "/base",
		},
		{
			name:     "both with slashes",
			basePath: "/base/",
			suffix:   "/api/v2/health",
			want:     "/base/api/v2/health",
		},
		{
			name:     "no slashes",
			basePath: "base",
			suffix:   "api/v2/health",
			want:     "/base/api/v2/health",
		},
		{
			name:     "with whitespace",
			basePath: " /base/ ",
			suffix:   " api/v2/health ",
			want:     "/base/api/v2/health",
		},
		{
			name:     "multiple slashes",
			basePath: "///base///",
			suffix:   "///api/v2/health///",
			want:     "/base/api/v2/health",
		},
		{
			name:     "base with trailing slash",
			basePath: "/base/",
			suffix:   "health",
			want:     "/base/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPublicEndpointPath(tt.basePath, tt.suffix)
			if got != tt.want {
				t.Errorf("BuildPublicEndpointPath(%q, %q) = %q, want %q", tt.basePath, tt.suffix, got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "empty path",
			path: "",
			want: "/",
		},
		{
			name: "root path",
			path: "/",
			want: "/",
		},
		{
			name: "no leading slash",
			path: "api/health",
			want: "/api/health",
		},
		{
			name: "with leading slash",
			path: "/api/health",
			want: "/api/health",
		},
		{
			name: "trailing slash",
			path: "/api/health/",
			want: "/api/health",
		},
		{
			name: "both leading and trailing slash",
			path: "/api/health/",
			want: "/api/health",
		},
		{
			name: "root with trailing slash",
			path: "/",
			want: "/",
		},
		{
			name: "single trailing slash removed",
			path: "/api/health/",
			want: "/api/health",
		},
		{
			name: "no slashes",
			path: "health",
			want: "/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.path)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
