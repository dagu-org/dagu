// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useQuery } from '@/hooks/api';
import { TemplateSelector } from '../TemplateSelector';

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

const appBarValue = {
  title: 'Cockpit',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const mockDags = [
  {
    fileName: 'example.yaml',
    dag: {
      name: 'Example DAG',
      group: 'main',
      tags: ['batch', 'workspace=ops'],
      description: 'Example workflow',
      params: [],
    },
    errors: [],
  },
];

const queryCalls: Array<{
  path: string;
  options?: { isPaused?: () => boolean };
}> = [];

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (
      path: string,
      params?: unknown,
      options?: { isPaused?: () => boolean }
    ) => unknown
  ) => void;
};

function latestQueryCall(path: string) {
  const calls = queryCalls.filter((call) => call.path === path);
  return calls[calls.length - 1];
}

function renderSelector(
  props?: Partial<React.ComponentProps<typeof TemplateSelector>>
) {
  return render(
    <AppBarContext.Provider value={appBarValue}>
      <TemplateSelector
        selectedTemplate=""
        selectedWorkspace=""
        onSelect={vi.fn()}
        {...props}
      />
    </AppBarContext.Provider>
  );
}

afterEach(() => {
  cleanup();
  queryCalls.length = 0;
  vi.clearAllMocks();
});

describe('TemplateSelector', () => {
  it('loads dags only while open and loads tags only when the tag filter is opened', () => {
    useQueryMock.mockImplementation((path, _params, options) => {
      queryCalls.push({ path, options });
      if (path === '/dags') {
        return { data: { dags: mockDags }, isLoading: false } as never;
      }
      if (path === '/dags/tags') {
        return { data: { tags: ['batch', 'workspace=ops'] } } as never;
      }
      return { data: undefined } as never;
    });

    renderSelector();

    expect(latestQueryCall('/dags')?.options?.isPaused?.()).toBe(true);
    expect(latestQueryCall('/dags/tags')?.options?.isPaused?.()).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: /select template/i }));

    expect(latestQueryCall('/dags')?.options?.isPaused?.()).toBe(false);
    expect(latestQueryCall('/dags/tags')?.options?.isPaused?.()).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: /tags/i }));

    expect(latestQueryCall('/dags/tags')?.options?.isPaused?.()).toBe(false);
  });

  it('keeps the selected template label after close without keeping /dags active', () => {
    useQueryMock.mockImplementation((path, _params, options) => {
      queryCalls.push({ path, options });
      if (path === '/dags') {
        return { data: { dags: mockDags }, isLoading: false } as never;
      }
      if (path === '/dags/tags') {
        return { data: { tags: ['batch', 'workspace=ops'] } } as never;
      }
      return { data: undefined } as never;
    });

    function StatefulSelector() {
      const [selectedTemplate, setSelectedTemplate] = React.useState('');
      return (
        <AppBarContext.Provider value={appBarValue}>
          <TemplateSelector
            selectedTemplate={selectedTemplate}
            selectedWorkspace=""
            onSelect={setSelectedTemplate}
          />
        </AppBarContext.Provider>
      );
    }

    render(<StatefulSelector />);

    fireEvent.click(screen.getByRole('button', { name: /select template/i }));
    fireEvent.click(screen.getByText('Example DAG'));

    expect(screen.queryByPlaceholderText('Search DAGs...')).not.toBeInTheDocument();
    expect(screen.getByText('Example DAG')).toBeInTheDocument();
    expect(latestQueryCall('/dags')?.options?.isPaused?.()).toBe(true);
  });
});
