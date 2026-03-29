// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import DAGRunDetailsPage from '..';
import { useBoundedDAGRunDetails } from '../../../../features/dag-runs/hooks/useBoundedDAGRunDetails';

vi.mock('@/contexts/PageContext', () => ({
  usePageContext: () => ({
    setContext: vi.fn(),
  }),
}));

vi.mock('@/contexts/WorkspaceContext', () => ({
  useWorkspace: () => ({
    selectedWorkspace: 'ops',
    workspaceReady: true,
  }),
}));

vi.mock('../../../../features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsContent: () => <div>dag run details</div>,
}));

vi.mock('../../../../features/dag-runs/hooks/useBoundedDAGRunDetails', () => ({
  useBoundedDAGRunDetails: vi.fn(),
}));

const useBoundedDAGRunDetailsMock = useBoundedDAGRunDetails as unknown as {
  mockReturnValue: (value: unknown) => void;
};

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/dag-runs/example/run-1']}>
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: vi.fn(),
          remoteNodes: ['local'],
          setRemoteNodes: vi.fn(),
          selectedRemoteNode: 'local',
          selectRemoteNode: vi.fn(),
        }}
      >
        <Routes>
          <Route
            path="/dag-runs/:name/:dagRunId"
            element={<DAGRunDetailsPage />}
          />
        </Routes>
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  useBoundedDAGRunDetailsMock.mockReturnValue({
    data: {
      name: 'example',
      dagRunId: 'run-1',
      tags: ['workspace=ops'],
    },
    error: null,
    refresh: vi.fn(),
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DAGRunDetailsPage workspace boundary', () => {
  it('renders DAG run details when the run matches the selected workspace', () => {
    renderPage();

    expect(screen.getByText('dag run details')).toBeInTheDocument();
  });

  it('renders a filtered-out state when the run does not match the selected workspace', () => {
    useBoundedDAGRunDetailsMock.mockReturnValue({
      data: {
        name: 'example',
        dagRunId: 'run-1',
        tags: ['workspace=other'],
      },
      error: null,
      refresh: vi.fn(),
    });

    renderPage();

    expect(screen.getByText('DAG Run Not Available')).toBeInTheDocument();
    expect(
      screen.getByText('This DAG run is outside the selected workspace.')
    ).toBeInTheDocument();
    expect(screen.queryByText('dag run details')).not.toBeInTheDocument();
  });
});
