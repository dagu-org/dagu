// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_UTF8RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "TwoByte", value: "é"},
		{name: "ThreeByte", value: "日本"},
		{name: "FourByte", value: "🙂"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := NewOutputMessage([]byte(tt.value))
			decoded, err := (&Message{Data: msg.Data}).DecodeData()
			require.NoError(t, err)
			assert.Equal(t, []byte(tt.value), decoded)
		})
	}
}
