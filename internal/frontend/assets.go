// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"embed"
)

var (
	//go:embed templates/* assets/*
	assetsFS embed.FS
)
