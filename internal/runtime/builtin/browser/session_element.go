// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// An element operation acts, without a model, on an element the outline
// names by its ID: a click, typing into a field, or picking an option. The
// session keeps it as the act a step writes, with the action it took as the
// act's recording, so the step replays exactly that action and heals by the
// act's words when the page changes.

// The kinds of element operation.
const (
	opClick  = "click"
	opType   = "type"
	opSelect = "select"
)

// elementOperation is one operation on an element the outline named.
type elementOperation struct {
	kind, element string
	// text is what type types, or the option select picks.
	text string
	// guards are its when and timeout, which its act takes as written.
	guards map[string]json.RawMessage
}

// elementGuards are what an element operation may carry besides its kind.
var elementGuards = []string{"when", "timeout"}

// parseElementOperation reads raw as an element operation, and reports
// false for any other operation.
func parseElementOperation(raw json.RawMessage) (elementOperation, bool, error) {
	var item map[string]json.RawMessage
	if json.Unmarshal(raw, &item) != nil {
		return elementOperation{}, false, nil
	}
	guards := map[string]json.RawMessage{}
	for _, name := range elementGuards {
		if value, ok := item[name]; ok {
			guards[name] = value
			delete(item, name)
		}
	}
	if len(item) != 1 {
		return elementOperation{}, false, nil
	}
	for kind, value := range item {
		op := elementOperation{kind: kind, guards: guards}
		var err error
		switch kind {
		case opClick:
			op.element, err = elementID(value)
		case opType:
			var in struct {
				Into json.RawMessage `json:"into"`
				Text string          `json:"text"`
			}
			if err = json.Unmarshal(value, &in); err == nil {
				op.element, err = elementID(in.Into)
				op.text = in.Text
			}
		case opSelect:
			var in struct {
				In     json.RawMessage `json:"in"`
				Option string          `json:"option"`
			}
			if err = json.Unmarshal(value, &in); err == nil {
				op.element, err = elementID(in.In)
				op.text = in.Option
			}
			if err == nil && op.text == "" {
				err = fmt.Errorf("select needs the option to pick")
			}
		default:
			return elementOperation{}, false, nil
		}
		if err != nil {
			return elementOperation{}, true, &SessionError{Code: CodeInvalidInput, Message: fmt.Sprintf(
				`%s: %v; name the element by the ID the outline shows before it, such as {"click": "0-131"}, {"type": {"into": "0-229", "text": "%%user%%"}}, or {"select": {"in": "0-106", "option": "未出荷"}}`, kind, err)}
		}
		return op, true, nil
	}
	return elementOperation{}, false, nil
}

// operation is the act the element operation stands for, carrying its
// guards, checked as any act is.
func (op elementOperation) operation(instruction string) (operation, error) {
	item := map[string]any{opAct: instruction}
	for name, value := range op.guards {
		item[name] = value
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return operation{}, err
	}
	return parseSessionOperation(raw)
}

// elementID reads an element's ID as the outline shows it, with or without
// its brackets.
func elementID(raw json.RawMessage) (string, error) {
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		var number json.Number
		if json.Unmarshal(raw, &number) != nil {
			return "", fmt.Errorf("the element ID must be text")
		}
		id = number.String()
	}
	id = strings.Trim(strings.TrimSpace(id), "[]")
	if id == "" {
		return "", fmt.Errorf("the element ID is missing")
	}
	return id, nil
}

// elementAct is the act an element operation stands for on the page snap
// shows: the act's instruction, and the action that does it, identifying
// the element with variables' values as their placeholders.
func elementAct(snap pageSnapshot, op elementOperation, variables map[string]string) (string, recordedAction, error) {
	roots := parseSnapshotTree(snap.Tree)
	path := pathTo(roots, op.element)
	xpath := snap.XPaths[op.element]
	if path == nil || xpath == "" {
		return "", recordedAction{}, fmt.Errorf("element %s is not on the page now; describe the page again for its elements", op.element)
	}
	target := pageTarget{roots: roots, path: path}
	node := target.node()
	if node.name == "" {
		return "", recordedAction{}, fmt.Errorf("element %s has no name a step could find it by again; act on it with an instruction that describes it", op.element)
	}
	selector := xpath
	if !strings.HasPrefix(selector, "xpath=") {
		selector = "xpath=" + selector
	}
	name := strconv.Quote(clip(node.name, outlineTextRunes))
	action := identified(recordedAction{Selector: selector, Description: withName(node.role, node.name)}, target, variables)
	switch op.kind {
	case opType:
		if !slices.Contains(fieldRoles, node.role) && node.role != "combobox" {
			return "", recordedAction{}, fmt.Errorf("element %s is a %s, which takes no typing", op.element, node.role)
		}
		action.Method, action.Arguments = "fill", []string{op.text}
		return fmt.Sprintf("Type %s into the %s field", op.text, name), action, nil
	case opSelect:
		if !slices.Contains(choiceRoles, node.role) {
			return "", recordedAction{}, fmt.Errorf("element %s is a %s, which has no options", op.element, node.role)
		}
		action.Method, action.Arguments = "selectOptionFromDropdown", []string{op.text}
		return fmt.Sprintf("Select %s in the %s dropdown", strconv.Quote(op.text), name), action, nil
	default:
		action.Method = "click"
		return fmt.Sprintf("Click the %s %s", name, roleWord(node.role)), action, nil
	}
}

// roleWord is what a person calls an element of role.
func roleWord(role string) string {
	switch role {
	case "radio":
		return "radio button"
	case "menuitem", "menuitemcheckbox", "menuitemradio":
		return "menu item"
	case "textbox", "searchbox", "spinbutton":
		return "field"
	case "treeitem":
		return "item"
	}
	return role
}
