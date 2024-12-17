// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package build

import "strings"

var (
	Version = "dev"
	AppName = "Dagu"
	Slug    = ""
)

func init() {
	if Slug == "" {
		Slug = strings.ToLower(AppName)
	}
}
