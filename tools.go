// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build tools

package tools

// This package keeps track of tool dependencies, see:
// https://github.com/golang/go/issues/25922
// https://www.jvt.me/posts/2022/06/15/go-tools-dependency-management/

import (
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/segmentio/golines"
	_ "github.com/yohamta/gomerger"
	_ "gotest.tools/gotestsum"
)
