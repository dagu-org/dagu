// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import "testing"

func TestParseTriggerTypeAcceptsLegacyAutomata(t *testing.T) {
	t.Parallel()

	if got := ParseTriggerType("automata"); got != TriggerTypeAutopilot {
		t.Fatalf("ParseTriggerType(automata) = %v, want %v", got, TriggerTypeAutopilot)
	}
}
