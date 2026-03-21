// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient, useQuery } from '@/hooks/api';
import { DAGPreviewModal } from '../DAGPreviewModal';

const mockStartModal = vi.fn((_props?: unknown) => <div>start modal</div>);

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@/features/dags/components/dag-execution/StartDAGModal', () => ({
  default: (props: unknown) => mockStartModal(props),
}));

const appBarValue = {
  title: 'Cockpit',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const useClientMock = useClient as unknown as {
  mockReturnValue: (value: unknown) => void;
};

const useQueryMock = useQuery as unknown as {
  mockReturnValue: (value: unknown) => void;
};

function renderPreview(selectedWorkspace = 'briefing/alpha') {
  return render(
    <AppBarContext.Provider value={appBarValue}>
      <DAGPreviewModal
        fileName="example"
        isOpen={true}
        selectedWorkspace={selectedWorkspace}
        onClose={vi.fn()}
      />
    </AppBarContext.Provider>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('DAGPreviewModal', () => {
  it('loads the preview from /dags/{fileName} and passes the DAG to the start modal', () => {
    useClientMock.mockReturnValue({
      POST: vi.fn(),
    } as never);
    useQueryMock.mockReturnValue({
      data: {
        dag: { name: 'Example DAG' },
        spec: 'steps:\n  - name: hello',
      },
      error: undefined,
      isLoading: false,
    } as never);

    renderPreview();

    expect(vi.mocked(useQuery)).toHaveBeenCalledWith(
      '/dags/{fileName}',
      expect.objectContaining({
        params: {
          path: { fileName: 'example' },
          query: { remoteNode: 'local' },
        },
      }),
      expect.any(Object)
    );
    expect(screen.getByText('Example DAG')).toBeInTheDocument();
    expect(screen.getByText(/steps:/)).toBeInTheDocument();
    expect(mockStartModal).toHaveBeenCalledWith(
      expect.objectContaining({
        dag: { name: 'Example DAG' },
        action: 'enqueue',
      })
    );
  });

  it('opens the start modal and enqueues with a sanitized workspace tag', async () => {
    const post = vi.fn().mockResolvedValue({
      data: { dagRunId: 'queued-run' },
      error: undefined,
    });
    useClientMock.mockReturnValue({ POST: post } as never);
    useQueryMock.mockReturnValue({
      data: {
        dag: { name: 'Example DAG' },
        spec: 'steps:\n  - name: hello',
      },
      error: undefined,
      isLoading: false,
    } as never);

    renderPreview();
    fireEvent.click(screen.getByRole('button', { name: /enqueue/i }));

    const props = mockStartModal.mock.calls[mockStartModal.mock.calls.length - 1]?.[0] as {
      visible: boolean;
      onSubmit: (
        params: string,
        dagRunId?: string
      ) => Promise<void>;
    };

    expect(props.visible).toBe(true);
    await expect(props.onSubmit('[\"x\"]', 'manual-run')).resolves.toBeUndefined();
    expect(post).toHaveBeenCalledWith('/dags/{fileName}/enqueue', {
      params: {
        path: { fileName: 'example' },
        query: { remoteNode: 'local' },
      },
      body: {
        params: '["x"]',
        dagRunId: 'manual-run',
        tags: ['workspace=briefingalpha'],
      },
    });
  });

  it('throws when the cockpit enqueue request fails', async () => {
    const post = vi.fn().mockResolvedValue({
      data: undefined,
      error: { message: 'enqueue failed' },
    });
    useClientMock.mockReturnValue({ POST: post } as never);
    useQueryMock.mockReturnValue({
      data: {
        dag: { name: 'Example DAG' },
        spec: 'steps:\n  - name: hello',
      },
      error: undefined,
      isLoading: false,
    } as never);

    renderPreview('ops');
    fireEvent.click(screen.getByRole('button', { name: /enqueue/i }));

    const props = mockStartModal.mock.calls[mockStartModal.mock.calls.length - 1]?.[0] as {
      onSubmit: () => Promise<void>;
    };

    await expect(props.onSubmit()).rejects.toThrow('enqueue failed');
  });
});
