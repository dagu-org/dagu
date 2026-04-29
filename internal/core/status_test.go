// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import "testing"

func TestParseTriggerTypeController(t *testing.T) {
	t.Parallel()

	if got := ParseTriggerType("controller"); got != TriggerTypeController {
		t.Fatalf("ParseTriggerType(controller) = %v, want %v", got, TriggerTypeController)
	}
}
