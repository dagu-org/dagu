// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { DAGPreviewModal } from '../DAGPreviewModal';

const mockSidePanel = vi.fn((props: unknown) => {
  void props;
  return <div>dag details panel</div>;
});

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
}));

vi.mock('@/features/dags/components/dag-details/DAGDetailsSidePanel', () => ({
  default: (props: unknown) => mockSidePanel(props),
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

const sidePanelMock = mockSidePanel as unknown as {
  mock: {
    calls: unknown[][];
  };
};

function renderPreview(
  props?: Partial<React.ComponentProps<typeof DAGPreviewModal>>
) {
  const onClose = props?.onClose ?? vi.fn();
  return render(
    <AppBarContext.Provider value={appBarValue}>
      <DAGPreviewModal
        fileName="example"
        isOpen={true}
        selectedWorkspace="briefing/alpha"
        onClose={onClose}
        {...props}
      />
    </AppBarContext.Provider>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('DAGPreviewModal', () => {
  it('renders the shared DAG details side panel with cockpit-specific props', () => {
    useClientMock.mockReturnValue({ POST: vi.fn() } as never);

    renderPreview();

    expect(mockSidePanel).toHaveBeenCalledWith(
      expect.objectContaining({
        fileName: 'example',
        isOpen: true,
        initialTab: 'status',
        renderInPortal: true,
        forceEnqueue: true,
        onClose: expect.any(Function),
        onEnqueue: expect.any(Function),
        toolbarHint: expect.anything(),
      })
    );
  });

  it('enqueues with a sanitized workspace label and returns the dag run id', async () => {
    const post = vi.fn().mockResolvedValue({
      data: { dagRunId: 'queued-run' },
      error: undefined,
    });
    const onClose = vi.fn();
    useClientMock.mockReturnValue({ POST: post } as never);

    renderPreview({ onClose });

    const props = sidePanelMock.mock.calls[
      sidePanelMock.mock.calls.length - 1
    ]?.[0] as unknown as {
      onEnqueue: (
        params: string,
        dagRunId?: string
      ) => Promise<string | void>;
    };

    await expect(props.onEnqueue('["x"]', 'manual-run')).resolves.toBe(
      'queued-run'
    );
    expect(post).toHaveBeenCalledWith('/dags/{fileName}/enqueue', {
      params: {
        path: { fileName: 'example' },
        query: { remoteNode: 'local' },
      },
      body: {
        params: '["x"]',
        dagRunId: 'manual-run',
        labels: ['workspace=briefingalpha'],
      },
    });
    expect(onClose).not.toHaveBeenCalled();
  });

  it('throws when the cockpit enqueue request fails', async () => {
    const post = vi.fn().mockResolvedValue({
      data: undefined,
      error: { message: 'enqueue failed' },
    });
    useClientMock.mockReturnValue({ POST: post } as never);

    renderPreview({ selectedWorkspace: 'ops' });

    const props = sidePanelMock.mock.calls[
      sidePanelMock.mock.calls.length - 1
    ]?.[0] as unknown as {
      onEnqueue: () => Promise<string | void>;
    };

    await expect(props.onEnqueue()).rejects.toThrow('enqueue failed');
  });
});
