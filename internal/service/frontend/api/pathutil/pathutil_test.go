// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package pathutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			suffix:   "api/v1/health",
			want:     "/api/v1/health",
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
			suffix:   "/api/v1/health",
			want:     "/base/api/v1/health",
		},
		{
			name:     "no slashes",
			basePath: "base",
			suffix:   "api/v1/health",
			want:     "/base/api/v1/health",
		},
		{
			name:     "with whitespace",
			basePath: " /base/ ",
			suffix:   " api/v1/health ",
			want:     "/base/api/v1/health",
		},
		{
			name:     "multiple slashes",
			basePath: "///base///",
			suffix:   "///api/v1/health///",
			want:     "/base/api/v1/health",
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

func TestBuildMountedAPIPath(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		apiBasePath string
		want        string
	}{
		{
			name:        "default api path",
			basePath:    "",
			apiBasePath: "/api/v1",
			want:        "/api/v1",
		},
		{
			name:        "empty api path falls back to default",
			basePath:    "",
			apiBasePath: "",
			want:        "/api/v1",
		},
		{
			name:        "nested base path",
			basePath:    "/dagu",
			apiBasePath: "/api/v1",
			want:        "/dagu/api/v1",
		},
		{
			name:        "custom api path without leading slash",
			basePath:    "/dagu",
			apiBasePath: "rest",
			want:        "/dagu/rest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMountedAPIPath(tt.basePath, tt.apiBasePath)
			assert.Equalf(t, tt.want, got, "BuildMountedAPIPath(%q, %q)", tt.basePath, tt.apiBasePath)
		})
	}
}

func TestBuildMountedAPIEndpointPath(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		apiBasePath string
		suffix      string
		want        string
	}{
		{
			name:        "default route",
			basePath:    "",
			apiBasePath: "/api/v1",
			suffix:      "openapi.json",
			want:        "/api/v1/openapi.json",
		},
		{
			name:        "nested base and custom api path",
			basePath:    "/dagu",
			apiBasePath: "/rest",
			suffix:      "/auth/login",
			want:        "/dagu/rest/auth/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMountedAPIEndpointPath(tt.basePath, tt.apiBasePath, tt.suffix)
			assert.Equalf(t, tt.want, got, "BuildMountedAPIEndpointPath(%q, %q, %q)", tt.basePath, tt.apiBasePath, tt.suffix)
		})
	}
}
