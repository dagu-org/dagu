// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlFieldRule struct {
	canonical string
	validate  func(node *yaml.Node, path string) error
}

func parseDefinitionYAML(data []byte, def *Definition) error {
	if def == nil {
		return errors.New("definition is required")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if len(doc.Content) == 0 {
		return errors.New("definition is required")
	}

	root := doc.Content[0]
	if isNullNode(root) {
		return errors.New("definition is required")
	}
	if err := validateDefinitionNode(root); err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, def); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	return nil
}

func validateDefinitionNode(node *yaml.Node) error {
	return validateMappingNode(node, "definition", map[string]yamlFieldRule{
		"kind":                 {},
		"nickname":             {},
		"iconUrl":              {canonical: "icon_url"},
		"icon_url":             {canonical: "icon_url"},
		"description":          {},
		"purpose":              {},
		"goal":                 {},
		"clonedFrom":           {canonical: "cloned_from"},
		"cloned_from":          {canonical: "cloned_from"},
		"standingInstruction":  {canonical: "standing_instruction"},
		"standing_instruction": {canonical: "standing_instruction"},
		"resetOnFinish":        {canonical: "reset_on_finish"},
		"reset_on_finish":      {canonical: "reset_on_finish"},
		"tags": {
			validate: validateStringListNode,
		},
		"schedule": {
			validate: validateScheduleNode,
		},
		"allowedDAGs": {
			canonical: "allowed_dags",
			validate:  validateAllowedDAGsNode,
		},
		"allowed_dags": {
			canonical: "allowed_dags",
			validate:  validateAllowedDAGsNode,
		},
		"agent": {
			validate: validateAgentConfigNode,
		},
		"disabled": {},
	})
}

func annotateClonedFromInSpec(spec, sourceName string) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &doc); err != nil {
		return "", fmt.Errorf("parse yaml: %w", err)
	}
	if len(doc.Content) == 0 || isNullNode(doc.Content[0]) {
		return "", errors.New("definition is required")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return "", errors.New("definition must be an object")
	}
	setMappingScalar(root, "cloned_from", sourceName, "clonedFrom")
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	return string(data), nil
}

func setMappingScalar(node *yaml.Node, key, value string, aliases ...string) {
	matchingKeys := map[string]struct{}{key: {}}
	for _, alias := range aliases {
		matchingKeys[alias] = struct{}{}
	}

	insertAt := -1
	nextContent := node.Content[:0]
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if _, ok := matchingKeys[strings.TrimSpace(keyNode.Value)]; ok {
			if insertAt == -1 {
				insertAt = len(nextContent)
			}
			continue
		}
		nextContent = append(nextContent, keyNode, node.Content[i+1])
	}
	node.Content = nextContent

	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
	if insertAt == -1 {
		insertAt = 0
		for i := 0; i < len(node.Content); i += 2 {
			if strings.TrimSpace(node.Content[i].Value) == "kind" {
				insertAt = i + 2
				break
			}
		}
	}
	node.Content = append(node.Content, nil, nil)
	copy(node.Content[insertAt+2:], node.Content[insertAt:])
	node.Content[insertAt] = keyNode
	node.Content[insertAt+1] = valueNode
}

func validateScheduleNode(node *yaml.Node, path string) error {
	if isNullNode(node) {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		return nil
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if child.Kind != yaml.ScalarNode {
				return fmt.Errorf("%s[%d] must be a string", path, i)
			}
		}
		return nil
	case yaml.DocumentNode, yaml.MappingNode, yaml.AliasNode:
		return fmt.Errorf("%s must be a string or list of strings", path)
	default:
		return fmt.Errorf("%s must be a string or list of strings", path)
	}
}

func validateAllowedDAGsNode(node *yaml.Node, path string) error {
	if isNullNode(node) {
		return nil
	}
	return validateMappingNode(node, path, map[string]yamlFieldRule{
		"names": {
			validate: validateStringListNode,
		},
		"tags": {
			validate: validateStringListNode,
		},
	})
}

func validateAgentConfigNode(node *yaml.Node, path string) error {
	if isNullNode(node) {
		return nil
	}
	return validateMappingNode(node, path, map[string]yamlFieldRule{
		"model":    {},
		"soul":     {},
		"safeMode": {},
	})
}

func validateStringListNode(node *yaml.Node, path string) error {
	if isNullNode(node) {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a list of strings", path)
	}
	for i, child := range node.Content {
		if child.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s[%d] must be a string", path, i)
		}
	}
	return nil
}

func validateMappingNode(node *yaml.Node, path string, rules map[string]yamlFieldRule) error {
	if isNullNode(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be an object", path)
	}

	seen := make(map[string]string, len(rules))
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := strings.TrimSpace(keyNode.Value)
		rule, ok := rules[key]
		if !ok {
			return fmt.Errorf("unknown field %q at %s", key, yamlNodePosition(keyNode))
		}
		canonical := rule.canonical
		if canonical == "" {
			canonical = key
		}
		if prev, ok := seen[canonical]; ok {
			return fmt.Errorf("duplicate field %q (already defined as %q) at %s", key, prev, yamlNodePosition(keyNode))
		}
		seen[canonical] = key
		if rule.validate != nil {
			if err := rule.validate(valueNode, path+"."+canonical); err != nil {
				return err
			}
		}
	}
	return nil
}

func isNullNode(node *yaml.Node) bool {
	if node == nil {
		return true
	}
	return node.Tag == "!!null" || (node.Kind == yaml.ScalarNode && node.Value == "")
}

func yamlNodePosition(node *yaml.Node) string {
	if node == nil || node.Line == 0 {
		return "unknown location"
	}
	if node.Column == 0 {
		return fmt.Sprintf("line %d", node.Line)
	}
	return fmt.Sprintf("line %d, column %d", node.Line, node.Column)
}
