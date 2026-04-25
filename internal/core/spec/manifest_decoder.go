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
)

type manifestDecoder struct {
	decodeHook mapstructure.DecodeHookFunc
}

func newManifestDecoder() *manifestDecoder {
	return &manifestDecoder{
		decodeHook: TypedUnionDecodeHook(),
	}
}

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

func (d *manifestDecoder) newMapDecoder(target *dag) (*mapstructure.Decoder, error) {
	return mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      target,
		TagName:     "yaml",
		DecodeHook:  d.decodeHook,
	})
}

func validateManifestAliases(input map[string]any) error {
	if _, hasLabels := input["labels"]; hasLabels {
		if _, hasTags := input["tags"]; hasTags {
			return fmt.Errorf("labels and deprecated tags cannot both be set")
		}
	}
	return nil
}
