// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type searchCursorFixture struct {
	Query string `json:"q"`
	Page  int    `json:"page"`
}

func TestEncodeSearchCursorRoundTrip(t *testing.T) {
	t.Parallel()

	raw := EncodeSearchCursor(searchCursorFixture{
		Query: "needle",
		Page:  2,
	})
	require.NotEmpty(t, raw)

	var decoded searchCursorFixture
	require.NoError(t, DecodeSearchCursor(raw, &decoded))
	assert.Equal(t, searchCursorFixture{Query: "needle", Page: 2}, decoded)
}

func TestDecodeSearchCursorRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	var decoded searchCursorFixture
	assert.ErrorIs(t, DecodeSearchCursor("%%%bad%%%", &decoded), ErrInvalidCursor)

	notJSON := EncodeSearchCursor("plain-string")
	assert.ErrorIs(t, DecodeSearchCursor(notJSON, &decoded), ErrInvalidCursor)
}
