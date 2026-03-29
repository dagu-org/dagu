// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package pathutil

import (
	"path"
	"strings"
)

// BuildPublicEndpointPath constructs a normalized endpoint path by combining
// a base path and suffix, ensuring the result starts with "/" and has no
// trailing slashes or duplicate separators.
func BuildPublicEndpointPath(basePath, suffix string) string {
	// Normalize both inputs: trim whitespace and slashes
	base := strings.Trim(strings.TrimSpace(basePath), "/")
	suffix = strings.Trim(strings.TrimSpace(suffix), "/")

	// Build the path using path.Join for consistent separator handling
	var fullPath string
	if base == "" {
		fullPath = suffix
	} else if suffix == "" {
		fullPath = base
	} else {
		fullPath = path.Join(base, suffix)
	}

	// Ensure the path starts with "/"
	if fullPath != "" && !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	return fullPath
}

// BuildMountedAPIPath constructs the effective mounted API path by combining
// the app base path and the configured API base path.
func BuildMountedAPIPath(basePath, apiBasePath string) string {
	if strings.TrimSpace(apiBasePath) == "" {
		apiBasePath = "/api/v1"
	}
	return BuildPublicEndpointPath(basePath, apiBasePath)
}

// BuildMountedAPIEndpointPath constructs a normalized endpoint path relative
// to the mounted API path.
func BuildMountedAPIEndpointPath(basePath, apiBasePath, suffix string) string {
	return BuildPublicEndpointPath(BuildMountedAPIPath(basePath, apiBasePath), suffix)
}

// NormalizePath ensures paths are comparable by enforcing a leading slash
// and trimming any trailing slash (except for root "/").
func NormalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
