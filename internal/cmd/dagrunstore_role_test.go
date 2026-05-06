// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/dagucloud/dagu/internal/persis/dagrunstore"
)

func TestDAGRunStoreRoleForCommand(t *testing.T) {
	tests := []struct {
		name string
		want dagrunstore.Role
	}{
		{name: "server", want: dagrunstore.RoleServer},
		{name: "start-all", want: dagrunstore.RoleServer},
		{name: "scheduler", want: dagrunstore.RoleScheduler},
		{name: "start", want: dagrunstore.RoleAgent},
		{name: "restart", want: dagrunstore.RoleAgent},
		{name: "retry", want: dagrunstore.RoleAgent},
		{name: "dry", want: dagrunstore.RoleAgent},
		{name: "exec", want: dagrunstore.RoleAgent},
		{name: "worker", want: dagrunstore.RoleAgent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dagRunStoreRoleForCommand(&cobra.Command{Use: tt.name}))
		})
	}
}
