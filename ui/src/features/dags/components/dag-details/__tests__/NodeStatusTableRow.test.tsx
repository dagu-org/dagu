// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { components } from '@/api/v1/schema';
import {
  NodeStatus,
  NodeStatusLabel,
  Status,
  StatusLabel,
} from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import { DAGContext } from '../../../contexts/DAGContext';
import NodeStatusTableRow from '../NodeStatusTableRow';

vi.mock('@/hooks/api', () => ({
  useClient: () => ({
    PATCH: vi.fn(),
    POST: vi.fn(),
  }),
  useQuery: vi.fn(),
}));

vi.mock('@/components/ui/error-modal', () => ({
  useErrorModal: () => ({
    showError: vi.fn(),
  }),
}));

const appBarValue = {
  title: 'DAGs',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const dagRun = {
  name: 'example',
  dagRunId: 'run-1',
  status: Status.Success,
  statusLabel: StatusLabel.succeeded,
  startedAt: '',
  finishedAt: '',
  autoRetryCount: 0,
} as components['schemas']['DAGRunDetails'];

const mockedUseQuery = vi.mocked(
  useQuery as unknown as (path: unknown, init: unknown) => unknown
);

describe('NodeStatusTableRow', () => {
  beforeEach(() => {
    mockedUseQuery.mockImplementation((_, init) => ({
      data: init ? { content: 'Deploying production\n' } : undefined,
      isLoading: false,
      error: undefined,
      isValidating: false,
      mutate: vi.fn(),
    }));
  });

  it('shows log step messages in the status table without opening step logs', () => {
    const node = {
      step: {
        name: 'announce',
        executorConfig: {
          type: 'log',
          config: {
            message: 'Deploying ${ENVIRONMENT}',
          },
        },
      },
      status: NodeStatus.Success,
      statusLabel: NodeStatusLabel.succeeded,
      stdout: '/tmp/announce.out',
      stderr: '',
      startedAt: '',
      finishedAt: '',
      retryCount: 0,
      doneCount: 1,
    } as components['schemas']['Node'];

    render(
      <MemoryRouter>
        <AppBarContext.Provider value={appBarValue}>
          <DAGContext.Provider
            value={{
              refresh: vi.fn(),
              name: 'example',
              fileName: 'example.yaml',
            }}
          >
            <table>
              <tbody>
                <NodeStatusTableRow
                  rownum={1}
                  node={node}
                  name="example.yaml"
                  dagRun={dagRun}
                  view="desktop"
                />
              </tbody>
            </table>
          </DAGContext.Provider>
        </AppBarContext.Provider>
      </MemoryRouter>
    );

    const message = screen.getByLabelText('Log message: Deploying production');
    expect(message).toBeInTheDocument();
    expect(message).toHaveClass('w-[320px]');
    expect(message.querySelector('svg')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Deploying ${ENVIRONMENT}')
    ).not.toBeInTheDocument();
    expect(screen.getByText('stdout')).toBeInTheDocument();
  });
});
