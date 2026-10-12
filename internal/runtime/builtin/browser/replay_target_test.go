// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ordersPage is an orders page's tree: a customer's link, a status select,
// and a button.
var ordersPage = pageSnapshot{
	Tree: "[0-1] RootWebArea: Orders\n  [0-2] link: ACME-1001\n  [0-3] select\n    [0-4] MenuListPopup\n      [0-5] option: すべて [selected]\n      [0-6] option: 未出荷\n  [0-7] button: Save\n",
	XPaths: map[string]string{
		"0-2": "/html[1]/body[1]/a[1]", "0-3": "/html[1]/body[1]/select[1]", "0-7": "/html[1]/body[1]/button[1]",
	},
}

// A replay goes ahead only while the element at the recorded location is
// the one the action recorded.
func TestReplayTarget(t *testing.T) {
	t.Parallel()

	customer := map[string]string{"customer": "ACME-1001"}
	for name, tc := range map[string]struct {
		action    recordedAction
		variables map[string]string
		want      bool
	}{
		"same element": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/button[1]", Method: "click", Role: "button", Name: "Save"},
			want:   true,
		},
		"another name": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/button[1]", Method: "click", Role: "button", Name: "Delete"},
		},
		"another role": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/a[1]", Method: "click", Role: "button", Name: "ACME-1001"},
		},
		"element gone": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/button[2]", Method: "click", Role: "button", Name: "Save"},
		},
		"name from a variable": {
			action:    recordedAction{Selector: "xpath=/html[1]/body[1]/a[1]", Method: "click", Role: "link", Name: "%customer%"},
			variables: customer,
			want:      true,
		},
		"name from another value": {
			action:    recordedAction{Selector: "xpath=/html[1]/body[1]/a[1]", Method: "click", Role: "link", Name: "%customer%"},
			variables: map[string]string{"customer": "ACME-1002"},
		},
		"select offering the option": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/select[1]", Method: selectOptionMethod, Arguments: []string{"未出荷"}, Role: "select"},
			want:   true,
		},
		"select lacking the option": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/select[1]", Method: selectOptionMethod, Arguments: []string{"出荷済み"}, Role: "select"},
		},
		"unidentified select lacking the option": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/select[1]", Method: selectOptionMethod, Arguments: []string{"出荷済み"}},
		},
		// A recording made before actions were identified replays as before.
		"unidentified element": {
			action: recordedAction{Selector: "xpath=/html[1]/body[1]/button[2]", Method: "click"},
			want:   true,
		},
	} {
		assert.Equal(t, tc.want, stillTargets(tc.action, ordersPage, tc.variables), name)
	}
}

// An action is identified by the element it targets, with a variable's
// value in the name kept as its placeholder.
func TestIdentifyActions(t *testing.T) {
	t.Parallel()

	actions := []recordedAction{
		{Selector: "xpath=/html[1]/body[1]/a[1]", Method: "click"},
		{Selector: "xpath=/html[1]/body[1]/div[1]", Method: "click"},
	}
	identifyOn(ordersPage, actions, map[string]string{"customer": "ACME-1001", "short": "AC"})

	require.Len(t, actions, 2)
	assert.Equal(t, "link", actions[0].Role)
	assert.Equal(t, "%customer%", actions[0].Name)
	assert.Empty(t, actions[1].Role, "an element outside the tree is not identified")
	assert.True(t, stillTargets(actions[0], ordersPage, map[string]string{"customer": "ACME-1001"}))
}
