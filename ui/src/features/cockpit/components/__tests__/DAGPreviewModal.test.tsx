// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { DAGPreviewModal } from '../DAGPreviewModal';

const mockSidePanel = vi.fn((_props?: unknown) => <div>shared side panel</div>);

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

function renderPreview(selectedWorkspace = 'briefing/alpha') {
  return render(
    <MemoryRouter>
      <AppBarContext.Provider value={appBarValue}>
        <DAGPreviewModal
          fileName="example"
          isOpen={true}
          selectedWorkspace={selectedWorkspace}
          onClose={vi.fn()}
        />
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('DAGPreviewModal', () => {
  it('passes cockpit-specific configuration to the shared side panel', () => {
    vi.mocked(useClient).mockReturnValue({
      POST: vi.fn(),
    } as never);

    renderPreview();

    expect(screen.getByText('shared side panel')).toBeInTheDocument();
    expect(mockSidePanel).toHaveBeenCalledWith(
      expect.objectContaining({
        fileName: 'example',
        isOpen: true,
        initialTab: 'spec',
        renderInPortal: true,
        backdropVisibleClassName: 'bg-black/5',
        forceEnqueue: true,
        onEnqueue: expect.any(Function),
      })
    );
  });

  it('enqueues with a sanitized workspace tag and returns the new dag run id', async () => {
    const post = vi.fn().mockResolvedValue({
      data: { dagRunId: 'queued-run' },
      error: undefined,
    });
    vi.mocked(useClient).mockReturnValue({ POST: post } as never);

    renderPreview();

    const props = mockSidePanel.mock.calls[mockSidePanel.mock.calls.length - 1]?.[0] as {
      onEnqueue: (
        params: string,
        dagRunId?: string
      ) => Promise<string | void>;
    };

    await expect(props.onEnqueue('[\"x\"]', 'manual-run')).resolves.toBe(
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
        tags: ['workspace=briefingalpha'],
      },
    });
  });

  it('throws when the cockpit enqueue request fails', async () => {
    const post = vi.fn().mockResolvedValue({
      data: undefined,
      error: { message: 'enqueue failed' },
    });
    vi.mocked(useClient).mockReturnValue({ POST: post } as never);

    renderPreview('ops');

    const props = mockSidePanel.mock.calls[mockSidePanel.mock.calls.length - 1]?.[0] as {
      onEnqueue: () => Promise<string | void>;
    };

    await expect(props.onEnqueue()).rejects.toThrow('enqueue failed');
  });
});
