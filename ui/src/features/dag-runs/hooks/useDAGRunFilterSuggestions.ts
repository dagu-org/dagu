import * as React from 'react';
import { Status } from '@/api/v1/schema';
import {
  type DAGRunListQuery,
  type DAGRunSummary,
  useExactDAGRuns,
} from './dagRunPagination';

export type DAGRunFilterSuggestionField = 'name' | 'dagRunId';

export type DAGRunFilterSuggestionFilters = {
  name: string;
  dagRunId: string;
  status: string;
  tags: string[];
  fromDate?: string;
  toDate?: string;
};

type UseDAGRunFilterSuggestionsOptions = {
  field: DAGRunFilterSuggestionField;
  filters: DAGRunFilterSuggestionFilters;
  remoteNode: string;
  isOpen: boolean;
  formatDateForApi: (dateString: string | undefined) => number | undefined;
  debounceMs?: number;
  maxSuggestions?: number;
};

type UseDAGRunFilterSuggestionsResult = {
  suggestions: string[];
  error: Error | null;
  isLoading: boolean;
};

const EMPTY_FILTERS: DAGRunFilterSuggestionFilters = {
  name: '',
  dagRunId: '',
  status: 'all',
  tags: [],
  fromDate: undefined,
  toDate: undefined,
};

const DEFAULT_DEBOUNCE_MS = 250;
const DEFAULT_MAX_SUGGESTIONS = 10;

function areTagsEqual(left: string[], right: string[]) {
  if (left.length !== right.length) {
    return false;
  }
  return left.every((tag, index) => tag === right[index]);
}

function areFiltersEqual(
  left: DAGRunFilterSuggestionFilters,
  right: DAGRunFilterSuggestionFilters
) {
  return (
    left.name === right.name &&
    left.dagRunId === right.dagRunId &&
    left.status === right.status &&
    areTagsEqual(left.tags, right.tags) &&
    left.fromDate === right.fromDate &&
    left.toDate === right.toDate
  );
}

export function buildDAGRunFilterSuggestionsQuery(
  filters: DAGRunFilterSuggestionFilters,
  remoteNode: string,
  formatDateForApi: (dateString: string | undefined) => number | undefined
): DAGRunListQuery {
  const name = filters.name.trim();
  const dagRunId = filters.dagRunId.trim();
  const status =
    filters.status !== 'all' ? (Number(filters.status) as Status) : undefined;

  return {
    remoteNode,
    name: name || undefined,
    dagRunId: dagRunId || undefined,
    status,
    tags: filters.tags.length > 0 ? filters.tags.join(',') : undefined,
    fromDate: formatDateForApi(filters.fromDate),
    toDate: formatDateForApi(filters.toDate),
  };
}

export function extractDAGNameSuggestions(
  dagRuns: DAGRunSummary[],
  maxSuggestions: number
): string[] {
  const uniqueNames = new Set<string>();
  for (const dagRun of dagRuns) {
    if (dagRun.name) {
      uniqueNames.add(dagRun.name);
    }
  }

  return [...uniqueNames]
    .sort((left, right) => {
      const normalizedCompare = left
        .toLowerCase()
        .localeCompare(right.toLowerCase());
      return normalizedCompare !== 0
        ? normalizedCompare
        : left.localeCompare(right);
    })
    .slice(0, maxSuggestions);
}

export function extractDAGRunIDSuggestions(
  dagRuns: DAGRunSummary[],
  maxSuggestions: number
): string[] {
  const uniqueIDs: string[] = [];
  const seen = new Set<string>();

  for (const dagRun of dagRuns) {
    if (!dagRun.dagRunId || seen.has(dagRun.dagRunId)) {
      continue;
    }
    seen.add(dagRun.dagRunId);
    uniqueIDs.push(dagRun.dagRunId);
    if (uniqueIDs.length >= maxSuggestions) {
      break;
    }
  }

  return uniqueIDs;
}

export function useDAGRunFilterSuggestions({
  field,
  filters,
  remoteNode,
  isOpen,
  formatDateForApi,
  debounceMs = DEFAULT_DEBOUNCE_MS,
  maxSuggestions = DEFAULT_MAX_SUGGESTIONS,
}: UseDAGRunFilterSuggestionsOptions): UseDAGRunFilterSuggestionsResult {
  const normalizedFilters = React.useMemo<DAGRunFilterSuggestionFilters>(
    () => ({
      name: filters.name.trim(),
      dagRunId: filters.dagRunId.trim(),
      status: filters.status,
      tags: [...filters.tags],
      fromDate: filters.fromDate,
      toDate: filters.toDate,
    }),
    [
      filters.dagRunId,
      filters.fromDate,
      filters.name,
      filters.status,
      filters.tags,
      filters.toDate,
    ]
  );
  const activeValue =
    field === 'name' ? normalizedFilters.name : normalizedFilters.dagRunId;
  const shouldFetch = isOpen && activeValue.length > 0;
  const [debouncedFilters, setDebouncedFilters] =
    React.useState<DAGRunFilterSuggestionFilters>(EMPTY_FILTERS);

  React.useEffect(() => {
    if (!shouldFetch) {
      setDebouncedFilters(EMPTY_FILTERS);
      return;
    }

    const timer = window.setTimeout(() => {
      setDebouncedFilters(normalizedFilters);
    }, debounceMs);

    return () => {
      window.clearTimeout(timer);
    };
  }, [debounceMs, normalizedFilters, shouldFetch]);

  const query = React.useMemo(
    () =>
      buildDAGRunFilterSuggestionsQuery(
        debouncedFilters,
        remoteNode,
        formatDateForApi
      ),
    [debouncedFilters, formatDateForApi, remoteNode]
  );
  const debouncedActiveValue =
    field === 'name' ? debouncedFilters.name : debouncedFilters.dagRunId;
  const isQueryReady = debouncedActiveValue.length > 0;
  const { data, error, isLoading, isValidating } = useExactDAGRuns({
    query,
    enabled: isQueryReady,
    liveEnabled: false,
  });

  const suggestions = React.useMemo(() => {
    if (!isQueryReady) {
      return [];
    }

    if (field === 'name') {
      return extractDAGNameSuggestions(data, maxSuggestions);
    }

    return extractDAGRunIDSuggestions(data, maxSuggestions);
  }, [data, field, isQueryReady, maxSuggestions]);

  return {
    suggestions,
    error,
    isLoading:
      shouldFetch &&
      (!areFiltersEqual(normalizedFilters, debouncedFilters) ||
        isLoading ||
        isValidating),
  };
}
