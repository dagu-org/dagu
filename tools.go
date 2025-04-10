//go:build tools

package tools

// This package keeps track of tool dependencies, see:
// https://github.com/golang/go/issues/25922
// https://www.jvt.me/posts/2022/06/15/go-tools-dependency-management/

import (
	_ "github.com/getkin/kin-openapi/cmd/validate"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/rhysd/changelog-from-release/v3"
	_ "github.com/segmentio/golines"
	_ "gotest.tools/gotestsum"
)
