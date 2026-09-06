// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/go-viper/mapstructure/v2"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// manifestDecoder converts raw YAML maps into internal manifest structures.
type manifestDecoder struct {
	decodeHook mapstructure.DecodeHookFunc
}

var defaultManifestDecoder = &manifestDecoder{
	decodeHook: typedUnionDecodeHook(),
}

// newManifestDecoder returns the shared manifest decoder instance.
func newManifestDecoder() *manifestDecoder {
	return defaultManifestDecoder
}

// Unmarshal decodes raw YAML bytes into a generic manifest map.
func (d *manifestDecoder) Unmarshal(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}

	parsed := make(map[string]any)
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&parsed); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}

	if len(parsed) == 0 {
		return nil, nil
	}
	if err := preserveEnvMappingOrder(data, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

// Decode converts a manifest map into the internal DAG representation.
func (d *manifestDecoder) Decode(input map[string]any) (*dag, error) {
	if err := validateManifestAliases(input); err != nil {
		return nil, err
	}

	decoded := new(dag)
	mapDecoder, err := d.newMapDecoder(decoded)
	if err != nil {
		return nil, err
	}

	if err := withLegacyKeyHint(mapDecoder.Decode(input)); err != nil {
		return nil, err
	}

	decoded.handlerOnRaw = extractRawHandlerOn(input)
	decoded.defaultsRaw = extractRawDefaults(input)
	return decoded, nil
}

// newMapDecoder creates a mapstructure decoder for the target manifest.
func (d *manifestDecoder) newMapDecoder(target *dag) (*mapstructure.Decoder, error) {
	return mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      target,
		TagName:     "yaml",
		DecodeHook:  d.decodeHook,
	})
}

// validateManifestAliases checks for incompatible legacy and current alias usage.
func validateManifestAliases(input map[string]any) error {
	if _, hasLabels := input["labels"]; hasLabels {
		if _, hasTags := input["tags"]; hasTags {
			return errors.New("labels and deprecated tags cannot both be set")
		}
	}
	return nil
}

func preserveEnvMappingOrder(data []byte, parsed map[string]any) error {
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return err
	}
	if file == nil {
		return nil
	}
	for _, doc := range file.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		return patchOrderedEnvValues(doc.Body, parsed)
	}
	return nil
}

// opaqueDataKeys holds step/DAG fields whose value is data owned by
// something other than dagu's own DAG/step schema: an executor's with:
// config, a step's or the DAG's own inline params: JSON Schema, and the
// legacy config: field. None of dagu's own env: declarations (DAG-level,
// defaults.env, step-level, or container.env) are ever nested inside one of
// these, but arbitrary executor- or schema-owned data underneath them can
// coincidentally use "env" as a field or property name (for example, a
// remote action's own with.env input, or a params: schema property named
// "env"). patchOrderedEnvValues must not descend into these keys, or it
// rewrites that unrelated mapping into dagu's ordered env-list shape too.
var opaqueDataKeys = map[string]bool{
	"with":   true,
	"params": true,
	"config": true,
}

func patchOrderedEnvValues(node ast.Node, target any) error {
	switch value := node.(type) {
	case *ast.MappingNode:
		targetMap, ok := target.(map[string]any)
		if !ok {
			return nil
		}
		for _, item := range value.Values {
			key, err := manifestNodeString(item.Key)
			if err != nil {
				return err
			}
			if key == "env" {
				envMap, ok := item.Value.(*ast.MappingNode)
				if ok {
					env, err := orderedEnvMappingRaw(envMap)
					if err != nil {
						return err
					}
					targetMap[key] = env
					continue
				}
			}
			if opaqueDataKeys[key] {
				continue
			}
			if err := patchOrderedEnvValues(item.Value, targetMap[key]); err != nil {
				return err
			}
		}
	case *ast.SequenceNode:
		targetSlice, ok := target.([]any)
		if !ok {
			return nil
		}
		for i, item := range value.Values {
			if i >= len(targetSlice) {
				break
			}
			if err := patchOrderedEnvValues(item, targetSlice[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func orderedEnvMappingRaw(node *ast.MappingNode) ([]any, error) {
	entries := make([]any, 0, len(node.Values))
	for _, item := range node.Values {
		key, err := manifestNodeString(item.Key)
		if err != nil {
			return nil, err
		}
		value, err := manifestNodeRaw(item.Value)
		if err != nil {
			return nil, err
		}
		entries = append(entries, map[string]any{key: value})
	}
	return entries, nil
}

func manifestNodeString(node ast.Node) (string, error) {
	raw, err := manifestNodeRaw(node)
	if err != nil {
		return "", err
	}
	switch value := raw.(type) {
	case string:
		return value, nil
	default:
		return fmt.Sprint(value), nil
	}
}

func manifestNodeRaw(node ast.Node) (any, error) {
	if node == nil {
		return nil, nil
	}
	if literal, ok := node.(*ast.LiteralNode); ok && literal.Value != nil {
		return literal.Value.Value, nil
	}
	data, err := node.MarshalYAML()
	if err != nil {
		return nil, err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
