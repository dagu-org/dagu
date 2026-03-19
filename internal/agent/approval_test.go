// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCommandApprovalWithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns approved and emits command approval prompt", func(t *testing.T) {
		t.Parallel()

		var emitted UserPrompt
		approved, err := requestCommandApprovalWithTimeout(
			context.Background(),
			func(prompt UserPrompt) { emitted = prompt },
			func(_ context.Context, promptID string) (UserPromptResponse, error) {
				assert.Equal(t, emitted.PromptID, promptID)
				return UserPromptResponse{
					PromptID:          promptID,
					SelectedOptionIDs: []string{"approve"},
				}, nil
			},
			"echo ok",
			"/tmp",
			"Approve command?",
			50*time.Millisecond,
		)

		require.NoError(t, err)
		assert.True(t, approved)
		assert.Equal(t, PromptTypeCommandApproval, emitted.PromptType)
		assert.Equal(t, "echo ok", emitted.Command)
		assert.Equal(t, "/tmp", emitted.WorkingDir)
		assert.Len(t, emitted.Options, 2)
	})

	t.Run("returns rejected without error", func(t *testing.T) {
		t.Parallel()

		approved, err := requestCommandApprovalWithTimeout(
			context.Background(),
			func(UserPrompt) {},
			func(_ context.Context, promptID string) (UserPromptResponse, error) {
				return UserPromptResponse{
					PromptID:          promptID,
					SelectedOptionIDs: []string{"reject"},
				}, nil
			},
			"echo ok",
			"/tmp",
			"Approve command?",
			50*time.Millisecond,
		)

		require.NoError(t, err)
		assert.False(t, approved)
	})

	t.Run("times out with timeout-specific error", func(t *testing.T) {
		t.Parallel()

		approved, err := requestCommandApprovalWithTimeout(
			context.Background(),
			func(UserPrompt) {},
			func(ctx context.Context, _ string) (UserPromptResponse, error) {
				<-ctx.Done()
				return UserPromptResponse{}, ctx.Err()
			},
			"echo ok",
			"/tmp",
			"Approve command?",
			10*time.Millisecond,
		)

		require.Error(t, err)
		assert.False(t, approved)
		assert.Contains(t, err.Error(), "approval timed out after 10ms")
	})

	t.Run("returns unavailable when approval channel is missing", func(t *testing.T) {
		t.Parallel()

		approved, err := requestCommandApprovalWithTimeout(
			context.Background(),
			nil,
			nil,
			"echo ok",
			"/tmp",
			"Approve command?",
			50*time.Millisecond,
		)

		require.Error(t, err)
		assert.False(t, approved)
		assert.Contains(t, err.Error(), "approval channel unavailable")
	})

	t.Run("passes through non-timeout wait errors", func(t *testing.T) {
		t.Parallel()

		approved, err := requestCommandApprovalWithTimeout(
			context.Background(),
			func(UserPrompt) {},
			func(_ context.Context, _ string) (UserPromptResponse, error) {
				return UserPromptResponse{}, assert.AnError
			},
			"echo ok",
			"/tmp",
			"Approve command?",
			50*time.Millisecond,
		)

		require.ErrorIs(t, err, assert.AnError)
		assert.False(t, approved)
	})
}
