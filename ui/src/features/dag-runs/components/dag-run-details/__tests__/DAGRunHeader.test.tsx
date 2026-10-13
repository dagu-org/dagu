// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel } from '@/api/v1/schema';
import { RemoteNodeProvider } from '@/contexts/RemoteNodeContext';
import DAGRunHeader from '../DAGRunHeader';

vi.mock('@/lib/clipboard', () => ({
  copyText: vi.fn(() => Promise.resolve(true)),
}));

vi.mock('@/contexts/ConfigContext', () => ({
  useConfig: () => ({ basePath: '' }),
}));

vi.mock('../../common', () => ({
  DAGRunActions: () => null,
}));

vi.mock('@/components/ui/matrix-text', () => ({
  MatrixText: ({ text }: { text: string }) => text,
}));

const dagRun = {
  name: 'nightly',
  dagRunId: 'run-42',
  rootDAGRunName: 'nightly',
  rootDAGRunId: 'run-42',
  status: Status.Success,
  statusLabel: StatusLabel.succeeded,
  startedAt: '',
  finishedAt: '',
  autoRetryCount: 0,
};

describe('DAGRunHeader', () => {
  it('announces the copy that happened last', async () => {
    render(
      <MemoryRouter>
        <RemoteNodeProvider remoteNode="local">
          <DAGRunHeader dagRun={dagRun as never} refreshFn={vi.fn()} />
        </RemoteNodeProvider>
      </MemoryRouter>
    );

    await act(async () => {
      fireEvent.click(
        screen.getByRole('button', { name: 'Copy link to this run' })
      );
    });
    expect(
      screen.getByText('Run link copied to clipboard')
    ).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy run ID' }));
    });
    expect(screen.getByText('Run ID copied to clipboard')).toBeInTheDocument();
    expect(
      screen.queryByText('Run link copied to clipboard')
    ).not.toBeInTheDocument();
  });
});
