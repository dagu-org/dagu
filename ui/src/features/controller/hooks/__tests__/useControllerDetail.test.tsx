// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { parse } from 'yaml';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@/features/agent/hooks/useAvailableModels', () => ({
  useAvailableModels: vi.fn(() => ({ models: [] })),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>(
    'react-router-dom'
  );
  return {
    ...actual,
    useNavigate: vi.fn(() => vi.fn()),
  };
});

import { useClient, useQuery } from '@/hooks/api';
import { useControllerDetailController } from '@/features/controller/hooks/useControllerDetail';

const useClientMock = useClient as unknown as {
  mockReturnValue: (value: unknown) => void;
};
const useQueryMock = useQuery as unknown as {
  mockImplementation: (fn: (...args: unknown[]) => unknown) => void;
};

describe('useControllerDetailController', () => {
  const putMock = vi.fn();
  const detailMutateMock = vi.fn(async () => undefined);
  const specMutateMock = vi.fn(async () => undefined);

  beforeEach(() => {
    putMock.mockReset();
    detailMutateMock.mockReset();
    specMutateMock.mockReset();

    useClientMock.mockReturnValue({
      PUT: putMock,
    });

    const detailData = {
      definition: {
        name: 'software_dev',
        description: 'Original description',
        iconUrl: '',
        goal: 'Ship it',
        resetOnFinish: false,
        trigger: {
          type: 'manual',
        },
        workflows: {
          names: ['build', 'run-tests'],
        },
        agent: {
          model: '',
        },
      },
      state: {
        state: 'idle',
        tasks: [],
      },
      taskTemplates: [],
      workflows: [
        { name: 'build', description: 'Build app', labels: [] },
        { name: 'run-tests', description: 'Run tests', labels: [] },
      ],
    };
    const specData = {
      spec: [
        'description: "Original description"',
        'goal: "Ship it"',
        'trigger:',
        '  type: "manual"',
        'workflows:',
        '  names:',
        '    - "build"',
        '    - "run-tests"',
        '',
      ].join('\n'),
    };

    useQueryMock.mockImplementation((path: unknown) => {
      if (path === '/controller/{name}') {
        return {
          data: detailData,
          error: undefined,
          isLoading: false,
          mutate: detailMutateMock,
        };
      }
      if (path === '/controller/{name}/spec') {
        return {
          data: specData,
          error: undefined,
          isLoading: false,
          mutate: specMutateMock,
        };
      }
      return {
        data: undefined,
        error: undefined,
        isLoading: false,
        mutate: vi.fn(async () => undefined),
      };
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('autosaves workflow name changes without saving unrelated metadata drafts', async () => {
    putMock.mockResolvedValue({ error: undefined });

    const { result } = renderHook(() =>
      useControllerDetailController({
        name: 'software_dev',
        enabled: true,
      })
    );

    act(() => {
      result.current.setDescriptionDraft('Unsaved local edit');
      result.current.setIsEditingMetadata(true);
    });

    await act(async () => {
      await result.current.onWorkflowNamesChange(['build']);
    });

    await waitFor(() => expect(putMock).toHaveBeenCalledTimes(1));

    const request = putMock.mock.calls[0]?.[1] as {
      body?: { spec?: string };
    };
    const nextSpec = parse(request.body?.spec || '');

    expect(nextSpec).toMatchObject({
      description: 'Original description',
      goal: 'Ship it',
      workflows: {
        names: ['build'],
      },
    });
    expect(detailMutateMock).toHaveBeenCalledTimes(1);
    expect(specMutateMock).toHaveBeenCalledTimes(1);
  });
});
