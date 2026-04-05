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
		"standingInstruction":  {canonical: "standing_instruction"},
		"standing_instruction": {canonical: "standing_instruction"},
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
		"model":         {},
		"soul":          {},
		"enabledSkills": {validate: validateStringListNode},
		"safeMode":      {},
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
