// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishedOutputContractValidatePath(t *testing.T) {
	t.Parallel()

	t.Run("DescendsTypedLiteralMaps", func(t *testing.T) {
		t.Parallel()

		contract := publishedOutputContract{
			StepName: "build",
			Source:   "output",
			Keys: map[string]StepOutputEntry{
				"artifact": {
					HasValue: true,
					Value: map[string]map[string]string{
						"meta": {"name": "report"},
					},
				},
			},
		}

		assert.Equal(t, outputReferenceValid, contract.validatePath([]string{"artifact", "meta", "name"}))
		assert.Equal(t, outputReferenceInvalid, contract.validatePath([]string{"artifact", "meta", "missing"}))
	})

	t.Run("TreatsUnresolvedRefSchemaAsUnknown", func(t *testing.T) {
		t.Parallel()

		contract := publishedOutputContract{
			StepName: "build",
			Source:   "output_schema",
			Schema: map[string]any{
				"$ref": "#/$defs/BuildOutput",
			},
		}

		tassert := assert.New(t)
		tassert.Equal(outputReferenceUnknown, contract.validatePath([]string{"artifact"}))
	})

	t.Run("TreatsEmptyCompositionAsUnknown", func(t *testing.T) {
		t.Parallel()

		for name, schema := range map[string]map[string]any{
			"empty anyOf":     {"anyOf": []any{}},
			"empty oneOf":     {"oneOf": []any{}},
			"empty allOf":     {"allOf": []any{}},
			"non-array anyOf": {"anyOf": "not-an-array"},
			"non-array oneOf": {"oneOf": "not-an-array"},
			"non-array allOf": {"allOf": "not-an-array"},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				contract := publishedOutputContract{
					StepName: "build",
					Source:   "output_schema",
					Schema:   schema,
				}

				assert.Equal(t, outputReferenceUnknown, contract.validatePath([]string{"artifact"}))
			})
		}
	})
}
