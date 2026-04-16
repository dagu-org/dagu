// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !race

package distr_test

func raceEnabled() bool {
	return false
}
