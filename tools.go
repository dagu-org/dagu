//go:build tools

package tools

// See for more details:
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/segmentio/golines"
	_ "github.com/yohamta/gomerger"
	_ "gotest.tools/gotestsum"
)
