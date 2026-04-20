// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useBoundedDAGRunDetails } from '@/features/dag-runs/hooks/useBoundedDAGRunDetails';
import ArtifactsTab from '@/features/dags/components/artifacts/ArtifactsTab';
import { ArtifactListModal } from '../ArtifactListModal';

vi.mock('@/features/dag-runs/hooks/useBoundedDAGRunDetails', () => ({
  useBoundedDAGRunDetails: vi.fn(),
}));

vi.mock('@/features/dags/components/artifacts/ArtifactsTab', () => ({
  default: vi.fn(({ dagRun }) => (
    <div data-testid="artifact-preview-tab">{dagRun.name}</div>
  )),
}));

const appBarValue = {
  title: 'Cockpit',
  setTitle: vi.fn(),
  remoteNodes: ['local', 'edge'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'edge',
  selectRemoteNode: vi.fn(),
};

const run = {
  dagRunId: 'run-1',
  name: 'artifact-dag',
  status: Status.Success,
  statusLabel: StatusLabel.succeeded,
  artifactsAvailable: true,
  autoRetryCount: 0,
  triggerType: TriggerType.manual,
  queuedAt: '',
  scheduleTime: '',
  startedAt: '2026-03-16T00:00:00Z',
  finishedAt: '2026-03-16T00:01:00Z',
};

const details = {
  ...run,
  rootDAGRunName: 'artifact-dag',
  rootDAGRunId: 'run-1',
  nodes: [],
};

afterEach(() => {
  vi.clearAllMocks();
});

describe('ArtifactListModal', () => {
  it('loads DAG-run details and renders the shared artifact preview tab', () => {
    vi.mocked(useBoundedDAGRunDetails).mockReturnValue({
      data: details,
      error: null,
      isLoading: false,
      isValidating: false,
      refresh: vi.fn(),
    } as never);

    render(
      <AppBarContext.Provider value={appBarValue}>
        <ArtifactListModal run={run} isOpen={true} onClose={() => {}} />
      </AppBarContext.Provider>
    );

    expect(screen.getByTestId('artifact-preview-tab')).toHaveTextContent(
      'artifact-dag'
    );
    expect(useBoundedDAGRunDetails).toHaveBeenCalledWith({
      target: {
        remoteNode: 'edge',
        name: 'artifact-dag',
        dagRunId: 'run-1',
      },
      enabled: true,
      pollIntervalMs: 2000,
    });
    expect(ArtifactsTab).toHaveBeenCalledWith(
      expect.objectContaining({
        dagRun: details,
        artifactEnabled: true,
        className: 'h-full',
        fillHeight: true,
      }),
      undefined
    );
  });
});
