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

const (
	// selectOptionMethod is the method of an action that picks an option of
	// a select.
	selectOptionMethod = "selectOptionFromDropdown"
	// nativeSelectRole is the role the tree gives a native select, which
	// shows all of its options.
	nativeSelectRole = "select"
	// contextRunes bounds the text kept around a repeated element.
	contextRunes = 200
)

// pageTarget is the element at an action's location on a page.
type pageTarget struct {
	roots []*outlineNode
	// path runs from a root of the tree down to the element.
	path []*outlineNode
}

// targetOf returns the element of snap at the XPath a selector names, and
// false when no node of the tree is there.
func targetOf(snap pageSnapshot, selector string) (pageTarget, bool) {
	xpath := strings.TrimPrefix(selector, "xpath=")
	for id, path := range snap.XPaths {
		if path != xpath {
			continue
		}
		roots := parseSnapshotTree(snap.Tree)
		if found := pathTo(roots, id); found != nil {
			return pageTarget{roots: roots, path: found}, true
		}
	}
	return pageTarget{}, false
}

// pathTo returns the nodes from one of nodes down to the node with id, or
// nil when none holds it.
func pathTo(nodes []*outlineNode, id string) []*outlineNode {
	for _, n := range nodes {
		if n.id == id {
			return []*outlineNode{n}
		}
		if path := pathTo(n.children, id); path != nil {
			return append([]*outlineNode{n}, path...)
		}
	}
	return nil
}

func (t pageTarget) node() *outlineNode { return t.path[len(t.path)-1] }

// distinctDepth is how many levels above the element its largest ancestor
// holding no other element of its role and name sits, such as its table
// row, or 0 when the page has no such other element.
func (t pageTarget) distinctDepth() int {
	node := t.node()
	if lookalikes(t.roots, node) <= 1 {
		return 0
	}
	depth := 0
	for i := len(t.path) - 2; i >= 0 && lookalikes(t.path[i:i+1], node) == 1; i-- {
		depth++
	}
	return depth
}

// context is the text of the element's ancestor depth levels up, with
// variables' values as their placeholders.
func (t pageTarget) context(depth int, variables map[string]string) string {
	if depth >= len(t.path) {
		return ""
	}
	return clip(withPlaceholders(textOf(t.path[len(t.path)-1-depth]), variables), contextRunes)
}

// lookalikes counts the nodes under nodes with the role and name of node.
func lookalikes(nodes []*outlineNode, node *outlineNode) int {
	count := 0
	for _, n := range nodes {
		if n.role == node.role && sameText(n.name, node.name) {
			count++
		}
		count += lookalikes(n.children, node)
	}
	return count
}

// identified returns action naming the element it targets, so a replay can
// tell the element is still there. An element with lookalikes on the page,
// such as a button in each row of a list, is also named by the text around
// it. A variable's value in either is replaced by its placeholder, so they
// match on a later run with another value.
func identified(action recordedAction, target pageTarget, variables map[string]string) recordedAction {
	node := target.node()
	action.Role = node.role
	action.Name = withPlaceholders(node.name, variables)
	if depth := target.distinctDepth(); depth > 0 {
		action.Context, action.ContextDepth = target.context(depth, variables), depth
	}
	return action
}

// identifyOn identifies each action by the element it targets on snap,
// the page before the actions ran. An action whose element is not in the
// tree is left unidentified.
func identifyOn(snap pageSnapshot, actions []recordedAction, variables map[string]string) {
	for i, action := range actions {
		if target, ok := targetOf(snap, action.Selector); ok {
			actions[i] = identified(action, target, variables)
		}
	}
}

// stillTargets reports whether the element at the location action recorded
// on snap is the one it acted on: the same role, name, and surrounding
// text, and for a select, one offering the option it picks. An action
// recorded without a role is checked for the option only.
func stillTargets(action recordedAction, snap pageSnapshot, variables map[string]string) bool {
	target, found := targetOf(snap, action.Selector)
	if action.Role != "" {
		if !found {
			return false
		}
		node := target.node()
		if node.role != action.Role || !sameText(withPlaceholders(node.name, variables), action.Name) {
			return false
		}
		if action.ContextDepth > 0 && target.context(action.ContextDepth, variables) != action.Context {
			return false
		}
	}
	return !found || offersOption(target.node(), action, variables)
}

// offersOption reports whether node, the element of a select action, has
// the option it picks. Any other action passes, as does an element that is
// not a native select and whose options the tree does not show.
func offersOption(node *outlineNode, action recordedAction, variables map[string]string) bool {
	if action.Method != selectOptionMethod || len(action.Arguments) == 0 {
		return true
	}
	options := optionsOf(node)
	if len(options) == 0 && node.role != nativeSelectRole {
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
