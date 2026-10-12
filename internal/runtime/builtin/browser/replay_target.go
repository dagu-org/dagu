// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/dagucloud/dagu/v2/internal/runtime/builtin/internal/agentstep"
)

// selectOptionMethod is the method of an action that picks an option of a
// select.
const selectOptionMethod = "selectOptionFromDropdown"

// targetOf returns the node of snap at the XPath a selector names, or nil
// when no node of the tree is there.
func targetOf(snap pageSnapshot, selector string) *outlineNode {
	xpath := strings.TrimPrefix(selector, "xpath=")
	for id, path := range snap.XPaths {
		if path == xpath {
			return findNode(parseSnapshotTree(snap.Tree), id)
		}
	}
	return nil
}

// identified returns action naming node, the element it targets, so a
// replay can tell the element is still there. A variable's value in the
// name is replaced by its placeholder, so the name matches on a later run
// with another value.
func identified(action recordedAction, node *outlineNode, variables map[string]string) recordedAction {
	action.Role = node.role
	action.Name = withPlaceholders(node.name, variables)
	return action
}

// identifyOn identifies each action by the element it targets on snap,
// the page before the actions ran. An action whose element is not in the
// tree is left unidentified.
func identifyOn(snap pageSnapshot, actions []recordedAction, variables map[string]string) {
	for i, action := range actions {
		if node := targetOf(snap, action.Selector); node != nil {
			actions[i] = identified(action, node, variables)
		}
	}
}

// stillTargets reports whether the element at the location action recorded
// on snap is the one it acted on: the same role and name, and for a select,
// one offering the option it picks. An action recorded without a role is
// checked for the option only.
func stillTargets(action recordedAction, snap pageSnapshot, variables map[string]string) bool {
	node := targetOf(snap, action.Selector)
	if action.Role != "" {
		if node == nil || node.role != action.Role || !sameText(node.name, agentstep.SubstituteVariables(action.Name, variables)) {
			return false
		}
	}
	return node == nil || offersOption(node, action, variables)
}

// offersOption reports whether node, the element of a select action, has
// the option it picks. Any other action, and an element whose options the
// tree does not show, passes.
func offersOption(node *outlineNode, action recordedAction, variables map[string]string) bool {
	options := optionsOf(node)
	if action.Method != selectOptionMethod || len(action.Arguments) == 0 || len(options) == 0 {
		return true
	}
	want := agentstep.SubstituteVariables(action.Arguments[0], variables)
	return slices.ContainsFunc(options, func(option *outlineNode) bool { return sameText(option.name, want) })
}

// withPlaceholders replaces each variable value of at least
// minRecordedValueRunes in text with its %name%, longer values first.
func withPlaceholders(text string, variables map[string]string) string {
	names := slices.SortedFunc(maps.Keys(variables), func(a, b string) int {
		return cmp.Or(len(variables[b])-len(variables[a]), strings.Compare(a, b))
	})
	for _, name := range names {
		if value := variables[name]; len([]rune(value)) >= minRecordedValueRunes {
			text = strings.ReplaceAll(text, value, "%"+name+"%")
		}
	}
	return text
}

// sameText reports whether two names are equal apart from their spacing.
func sameText(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}
