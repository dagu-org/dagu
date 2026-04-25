// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"encoding/json"
	"github.com/dagucloud/dagu/internal/core/exec"
)

type pushBackPayload struct {
	Iteration int                  `json:"iteration"`
	Inputs    map[string]string    `json:"inputs,omitempty"`
	History   []exec.PushBackEntry `json:"history,omitempty"`
}

func marshalPushBackPayload(allowedInputs []string, state NodeState) (string, error) {
	if state.ApprovalIteration == 0 {
		return "", nil
	}

	payload := pushBackPayload{
		Iteration: state.ApprovalIteration,
		Inputs:    exec.FilterPushBackInputs(allowedInputs, state.PushBackInputs),
		History:   exec.NormalizePushBackHistory(allowedInputs, state.ApprovalIteration, state.PushBackInputs, state.PushBackHistory),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
