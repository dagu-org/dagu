// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"embed"
)

var (
	//go:embed templates/* assets/*
	assetsFS embed.FS
)
