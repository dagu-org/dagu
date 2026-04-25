// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"bytes"
	"errors"
	"io"

	"github.com/go-viper/mapstructure/v2"
	"github.com/goccy/go-yaml"
)

// manifestDecoder converts raw YAML maps into internal manifest structures.
type manifestDecoder struct {
	decodeHook mapstructure.DecodeHookFunc
}

var defaultManifestDecoder = &manifestDecoder{
	decodeHook: TypedUnionDecodeHook(),
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

	if err := withSnakeCaseKeyHint(mapDecoder.Decode(input)); err != nil {
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
