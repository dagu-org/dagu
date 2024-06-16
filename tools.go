//go:build tools

package dagu

// See for more details:
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
