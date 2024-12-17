// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"

	"github.com/dagu-org/dagu/cmd"
	"github.com/dagu-org/dagu/internal/build"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}

}

var version = "0.0.0"

func init() {
	build.Version = version
}
