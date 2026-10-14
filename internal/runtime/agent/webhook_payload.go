// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
)

const (
	webhookPayloadParam = "WEBHOOK_PAYLOAD"
	webhookPayloadFile  = "WEBHOOK_PAYLOAD_FILE"
)

// webhookPayload gives a webhook run's steps the body as a file, since one
// environment variable holds only so much. It returns the params the steps
// see, leaving WEBHOOK_PAYLOAD out when it would not fit in a variable, the
// WEBHOOK_PAYLOAD_FILE entry, and a cleanup that removes the file. A run
// without a webhook body gets params as they are and no entry.
func webhookPayload(params []string) (stepParams []string, env string, cleanup func(), err error) {
	i := slices.IndexFunc(params, func(p string) bool { return strings.HasPrefix(p, webhookPayloadParam+"=") })
	if i < 0 {
		return params, "", func() {}, nil
	}
	dir, err := os.MkdirTemp("", "dagu-webhook-")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create webhook payload directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "payload")
	payload := strings.TrimPrefix(params[i], webhookPayloadParam+"=")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("failed to write webhook payload file: %w", err)
	}
	if len(params[i]) >= maxEnvEntry() {
		params = slices.Delete(slices.Clone(params), i, i+1)
	}
	return params, webhookPayloadFile + "=" + path, cleanup, nil
}

// maxEnvEntry bounds one NAME=VALUE environment entry: Linux refuses to start
// a process with an entry of 128 KiB or more, and Windows caps a variable at
// 32,767 characters. macOS is held to the Linux bound, well inside its limit
// on the whole environment.
func maxEnvEntry() int {
	if goruntime.GOOS == "windows" {
		return 32_767
	}
	return 128 << 10
}
