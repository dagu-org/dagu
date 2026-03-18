// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package coordinator

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestClientCacheUsesDerivedKeyForEmptyCoordinatorIDs(t *testing.T) {
	t.Parallel()

	cli := &clientImpl{
		config:  DefaultConfig(),
		clients: make(map[string]*client),
	}

	member1 := exec.HostInfo{Host: "127.0.0.1", Port: 1234}
	member2 := exec.HostInfo{Host: "127.0.0.1", Port: 5678}

	client1, err := cli.getOrCreateClient(member1)
	require.NoError(t, err)

	client2, err := cli.getOrCreateClient(member2)
	require.NoError(t, err)

	require.NotSame(t, client1, client2)
	require.Len(t, cli.clients, 2)
	require.Contains(t, cli.clients, coordinatorMemberKey(member1))
	require.Contains(t, cli.clients, coordinatorMemberKey(member2))

	cli.removeClient(member1)
	require.Len(t, cli.clients, 1)
	require.NotContains(t, cli.clients, coordinatorMemberKey(member1))
	require.Contains(t, cli.clients, coordinatorMemberKey(member2))

	cli.removeClient(member2)
	require.Empty(t, cli.clients)
}
