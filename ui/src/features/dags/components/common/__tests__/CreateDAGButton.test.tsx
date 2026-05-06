// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { WorkspaceKind } from '@/lib/workspace';
import CreateDAGButton from '../CreateDAGButton';

const { postMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
}));

vi.mock('@/contexts/AuthContext', () => ({
  useCanWrite: () => true,
}));

vi.mock('@/hooks/api', () => ({
  useClient: () => ({
    POST: postMock,
  }),
}));

describe('CreateDAGButton', () => {
  it('creates new DAGs with the selected workspace label', async () => {
    postMock.mockResolvedValueOnce({
      error: { message: 'stop before redirect' },
    });

    render(
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: vi.fn(),
          remoteNodes: ['local'],
          setRemoteNodes: vi.fn(),
          selectedRemoteNode: 'local',
          selectRemoteNode: vi.fn(),
          workspaces: [{ id: 'workspace-1', name: 'ops' }],
          workspaceSelection: {
            kind: WorkspaceKind.workspace,
            workspace: 'ops',
          },
          selectWorkspace: vi.fn(),
        }}
      >
        <CreateDAGButton />
      </AppBarContext.Provider>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Create new DAG' }));
    fireEvent.change(screen.getByLabelText('DAG Name'), {
      target: { value: 'header-new-dag' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(postMock).toHaveBeenCalled());

    expect(postMock).toHaveBeenCalledWith('/dags', {
      params: {
        query: {
          remoteNode: 'local',
        },
      },
      body: {
        name: 'header-new-dag',
        spec: expect.stringContaining('workspace=ops'),
      },
    });
  });
});
