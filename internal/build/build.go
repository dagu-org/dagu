// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package build

import "strings"

var (
	Version = ""
	AppName = "Dagu"
	Slug    = ""
)

func init() {
	if Slug == "" {
		Slug = strings.ToLower(AppName)
	}
}
