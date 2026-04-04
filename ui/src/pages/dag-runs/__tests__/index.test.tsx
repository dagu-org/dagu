// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import dayjs from 'dayjs';
import { Status } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { SearchStateProvider } from '@/contexts/SearchStateContext';
import { UserPreferencesProvider } from '@/contexts/UserPreference';
import { usePaginatedDAGRuns } from '@/features/dag-runs/hooks/dagRunPagination';
import { useDAGRunFilterSuggestions } from '@/features/dag-runs/hooks/useDAGRunFilterSuggestions';
import { useQuery } from '@/hooks/api';
import DAGRunsPage from '../index';

type SuggestionCall = Parameters<typeof useDAGRunFilterSuggestions>[0];

const paginatedCalls: Array<Record<string, unknown>> = [];
const suggestionCalls: SuggestionCall[] = [];

vi.mock('@/hooks/api', () => ({
  useQuery: vi.fn(),
}));

vi.mock('@/features/dag-runs/hooks/dagRunPagination', () => ({
  usePaginatedDAGRuns: vi.fn(),
}));

vi.mock('@/features/dag-runs/hooks/useDAGRunFilterSuggestions', () => ({
  useDAGRunFilterSuggestions: vi.fn(),
}));

vi.mock('@/features/dag-runs/components/common/DAGRunBatchActions', () => ({
  __esModule: true,
  default: () => null,
}));

vi.mock('@/features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: () => null,
}));

vi.mock('@/features/dag-runs/components/dag-run-list/DAGRunTable', () => ({
  __esModule: true,
  default: ({ dagRuns }: { dagRuns: unknown[] }) => (
    <div data-testid="dag-runs-table-count">{dagRuns.length}</div>
  ),
}));

vi.mock(
  '@/features/dag-runs/components/dag-run-list/DAGRunGroupedView',
  () => ({
    __esModule: true,
    default: ({ dagRuns }: { dagRuns: unknown[] }) => (
      <div data-testid="dag-runs-grouped-count">{dagRuns.length}</div>
    ),
  })
);

vi.mock('@/components/ui/select', async () => {
  const ReactModule = await import('react');
  const SelectContext = ReactModule.createContext<{
    value: string;
    onValueChange: (value: string) => void;
  } | null>(null);

  return {
    Select: ({
      value,
      onValueChange,
      children,
    }: {
      value: string;
      onValueChange: (value: string) => void;
      children: React.ReactNode;
    }) => (
      <SelectContext.Provider value={{ value, onValueChange }}>
        <div>{children}</div>
      </SelectContext.Provider>
    ),
    SelectTrigger: ({ children, className }: React.ComponentProps<'div'>) => (
      <div className={className}>{children}</div>
    ),
    SelectValue: ({ children }: { children?: React.ReactNode }) => (
      <div>{children}</div>
    ),
    SelectContent: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
    SelectItem: ({
      value,
      children,
    }: {
      value: string;
      children: React.ReactNode;
    }) => {
      const context = ReactModule.useContext(SelectContext);
      return (
        <button type="button" onClick={() => context?.onValueChange(value)}>
          {children}
        </button>
      );
    },
  };
});

vi.mock('@/components/ui/toggle-group', () => ({
  ToggleGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ToggleButton: ({
    children,
    onClick,
    value,
    groupValue,
  }: {
    children: React.ReactNode;
    onClick: () => void;
    value: string;
    groupValue: string;
  }) => (
    <button
      type="button"
      data-selected={groupValue === value}
      onClick={onClick}
    >
      {children}
    </button>
  ),
}));

vi.mock('@/components/ui/tag-combobox', () => ({
  TagCombobox: ({
    selectedTags,
    onTagsChange,
  }: {
    selectedTags: string[];
    onTagsChange: (tags: string[]) => void;
  }) => (
    <div>
      <button
        type="button"
        onClick={() =>
          onTagsChange(
            selectedTags.includes('critical')
              ? selectedTags
              : [...selectedTags, 'critical']
          )
        }
      >
        Add critical tag
      </button>
      <div data-testid="selected-tags">{selectedTags.join(',')}</div>
    </div>
  ),
}));

vi.mock('@/components/ui/date-range-picker', () => ({
  DateRangePicker: ({
    fromDate,
    toDate,
    onFromDateChange,
    onToDateChange,
    onEnterPress,
  }: {
    fromDate?: string;
    toDate?: string;
    onFromDateChange: (value: string) => void;
    onToDateChange: (value: string) => void;
    onEnterPress?: () => void;
  }) => (
    <div>
      <input
        aria-label="From date"
        value={fromDate ?? ''}
        onChange={(event) => onFromDateChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            onEnterPress?.();
          }
        }}
      />
      <input
        aria-label="To date"
        value={toDate ?? ''}
        onChange={(event) => onToDateChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            onEnterPress?.();
          }
        }}
      />
    </div>
  ),
}));

const useQueryMock = useQuery as unknown as {
  mockImplementation: (
    fn: (path: string, init: unknown, options: unknown) => unknown
  ) => void;
};

const usePaginatedDAGRunsMock = usePaginatedDAGRuns as unknown as {
  mockImplementation: (
    fn: (args: { query: Record<string, unknown> }) => unknown
  ) => void;
};

const useDAGRunFilterSuggestionsMock =
  useDAGRunFilterSuggestions as unknown as {
    mockImplementation: (fn: (args: SuggestionCall) => unknown) => void;
  };

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    apiURL: '/api/v1',
    basePath: '/',
    title: 'Dagu',
    navbarColor: '',
    tz: 'UTC',
    tzOffsetInSec: 0,
    version: 'test',
    maxDashboardPageLimit: 100,
    remoteNodes: 'local',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    agentEnabled: false,
    updateAvailable: false,
    latestVersion: '',
    permissions: {
      writeDags: true,
      runDags: true,
    },
    license: {
      valid: true,
      plan: 'community',
      expiry: '',
      features: [],
      gracePeriod: false,
      community: true,
      source: 'test',
      warningCode: '',
    },
    paths: {
      dagsDir: '',
      logDir: '',
      suspendFlagsDir: '',
      adminLogsDir: '',
      baseConfig: '',
      dagRunsDir: '',
      queueDir: '',
      procDir: '',
      serviceRegistryDir: '',
      configFileUsed: '',
      gitSyncDir: '',
      auditLogsDir: '',
    },
    ...overrides,
  };
}

function latestPaginatedCall(): Record<string, unknown> | undefined {
  return paginatedCalls[paginatedCalls.length - 1];
}

function latestSuggestionCall(
  field: SuggestionCall['field']
): SuggestionCall | undefined {
  return [...suggestionCalls]
    .reverse()
    .find((call) => call.field === field && call.isOpen);
}

function renderPage(initialEntry = '/dag-runs') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <ConfigContext.Provider value={makeConfig()}>
        <SearchStateProvider>
          <UserPreferencesProvider>
            <AppBarContext.Provider
              value={{
                title: '',
                setTitle: () => undefined,
                remoteNodes: ['local'],
                setRemoteNodes: () => undefined,
                selectedRemoteNode: 'local',
                selectRemoteNode: () => undefined,
              }}
            >
              <DAGRunsPage />
            </AppBarContext.Provider>
          </UserPreferencesProvider>
        </SearchStateProvider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('DAGRunsPage', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    paginatedCalls.length = 0;
    suggestionCalls.length = 0;
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });

    useQueryMock.mockImplementation((path) => {
      if (path === '/dags/tags') {
        return {
          data: { tags: ['critical'] },
          error: undefined,
          isLoading: false,
        } as never;
      }

      return {
        data: undefined,
        error: undefined,
        isLoading: false,
      } as never;
    });

    usePaginatedDAGRunsMock.mockImplementation(({ query }) => {
      paginatedCalls.push(query);
      return {
        dagRuns: [],
        isInitialLoading: false,
        isLoadingMore: false,
        loadMoreError: null,
        hasMore: false,
        refresh: vi.fn(),
        loadMore: vi.fn(),
      } as never;
    });

    useDAGRunFilterSuggestionsMock.mockImplementation((args) => {
      suggestionCalls.push(args);

      if (
        !args.isOpen ||
        (args.field === 'name' && !args.filters.name.trim()) ||
        (args.field === 'dagRunId' && !args.filters.dagRunId.trim())
      ) {
        return {
          suggestions: [],
          error: null,
          isLoading: false,
        };
      }

      if (args.field === 'name') {
        return {
          suggestions: ['payments', 'payments-monthly'],
          error: null,
          isLoading: false,
        };
      }

      return {
        suggestions: ['run-9', 'run-8'],
        error: null,
        isLoading: false,
      };
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders suggestions, updates only draft filters on selection, and waits for Search to refresh the table query', async () => {
    const user = userEvent.setup();

    renderPage();

    const nameInput = screen.getByPlaceholderText('Filter by DAG name...');
    await user.type(nameInput, 'pay');

    expect(
      await screen.findByRole('option', { name: 'payments' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('option', { name: 'payments' }));

    expect(nameInput).toHaveValue('payments');
    expect(latestPaginatedCall()?.name).toBeUndefined();

    await user.click(screen.getByRole('button', { name: /search/i }));

    await waitFor(() => {
      expect(latestPaginatedCall()?.name).toBe('payments');
    });
  });

  it('keeps status changes immediate without committing unrelated draft text filters', async () => {
    const user = userEvent.setup();

    renderPage();

    await user.type(
      screen.getByPlaceholderText('Filter by DAG name...'),
      'draft-dag'
    );
    await user.type(
      screen.getByPlaceholderText('Filter by Run ID...'),
      'draft-run'
    );
    await user.click(screen.getByText('failed'));

    await waitFor(() => {
      expect(latestPaginatedCall()).toEqual(
        expect.objectContaining({
          status: Status.Failed,
          name: undefined,
          dagRunId: undefined,
        })
      );
    });

    expect(screen.getByPlaceholderText('Filter by DAG name...')).toHaveValue(
      'draft-dag'
    );
    expect(screen.getByPlaceholderText('Filter by Run ID...')).toHaveValue(
      'draft-run'
    );
  });

  it('uses custom date drafts for suggestions without refreshing the table query until submit', async () => {
    const user = userEvent.setup();

    renderPage();

    const initialFromDate = latestPaginatedCall()?.fromDate;

    await user.click(screen.getByRole('button', { name: 'Custom' }));
    fireEvent.change(screen.getByLabelText('From date'), {
      target: { value: '2026-04-01T10:00' },
    });
    await user.type(
      screen.getByPlaceholderText('Filter by DAG name...'),
      'pay'
    );

    expect(
      await screen.findByRole('option', { name: 'payments' })
    ).toBeInTheDocument();
    expect(latestSuggestionCall('name')?.filters.fromDate).toBe(
      '2026-04-01T10:00'
    );
    expect(latestPaginatedCall()?.fromDate).toBe(initialFromDate);

    await user.click(screen.getByRole('button', { name: /search/i }));

    await waitFor(() => {
      expect(latestPaginatedCall()?.fromDate).toBe(
        dayjs('2026-04-01T10:00:00').unix()
      );
    });
  });

  it('keeps specific date filters immediate', async () => {
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: 'Specific' }));
    fireEvent.change(screen.getByDisplayValue(dayjs().format('YYYY-MM-DD')), {
      target: { value: '2026-04-03' },
    });

    await waitFor(() => {
      expect(latestPaginatedCall()).toEqual(
        expect.objectContaining({
          fromDate: dayjs('2026-04-03T00:00:00').unix(),
          toDate: dayjs('2026-04-03T23:59:00').unix(),
        })
      );
    });
  });
});
