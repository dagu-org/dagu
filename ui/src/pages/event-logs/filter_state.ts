import { ComponentsParametersEventLogPaginationMode } from '@/api/v1/schema';
import {
  createDefaultEventLogFilters,
  formatDateForApi,
  parseDateFromUrl,
} from './date_range';
import {
  isEventKindFilter,
  sanitizeEventTypeForKind,
} from './options';
import {
  PAGE_SIZE,
  SEARCH_STATE_KEY,
  type EventLogFilters,
  type EventLogQueryParams,
  type StoredEventLogState,
} from './types';
import { hasQueryParams } from './utils';

export type SearchStateStore = {
  readState<T>(pageKey: string, remoteKey?: string): T | undefined;
  writeState<T>(pageKey: string, remoteKey: string | undefined, value: T): void;
};

export function sanitizeFilters(filters: EventLogFilters): EventLogFilters {
  return {
    ...filters,
    type: sanitizeEventTypeForKind(filters.kind, filters.type),
  };
}

export function buildLocationParams(filters: EventLogFilters): URLSearchParams {
  const nextFilters = sanitizeFilters(filters);
  const params = new URLSearchParams();
  if (nextFilters.kind !== 'all') {
    params.set('kind', nextFilters.kind);
  }
  if (nextFilters.type !== 'all') {
    params.set('type', nextFilters.type);
  }
  if (nextFilters.dagName) {
    params.set('dagName', nextFilters.dagName);
  }
  if (nextFilters.automataName) {
    params.set('automataName', nextFilters.automataName);
  }
  if (nextFilters.dagRunId) {
    params.set('dagRunId', nextFilters.dagRunId);
  }
  if (nextFilters.attemptId) {
    params.set('attemptId', nextFilters.attemptId);
  }
  if (nextFilters.fromDate) {
    params.set('fromDate', nextFilters.fromDate);
  }
  if (nextFilters.toDate) {
    params.set('toDate', nextFilters.toDate);
  }
  params.set('dateMode', nextFilters.dateRangeMode);
  params.set('preset', nextFilters.datePreset);
  params.set('specificPeriod', nextFilters.specificPeriod);
  params.set('specificValue', nextFilters.specificValue);
  return params;
}

export function parseLocationState(args: {
  params: URLSearchParams;
  remoteKey: string;
  searchState: SearchStateStore;
  tzOffsetInSec?: number;
}): StoredEventLogState {
  const { params, remoteKey, searchState, tzOffsetInSec } = args;
  const persisted = searchState.readState<StoredEventLogState>(
    SEARCH_STATE_KEY,
    remoteKey
  );
  const base = sanitizeFilters({
    ...createDefaultEventLogFilters(tzOffsetInSec),
    ...persisted,
  });

  if (!hasQueryParams(params)) {
    return base;
  }

  const next: StoredEventLogState = { ...base };
  const kind = params.get('kind');
  if (isEventKindFilter(kind)) {
    next.kind = kind;
  }
  if (params.has('type')) {
    next.type = params.get('type') || 'all';
  }
  if (params.has('dagName')) {
    next.dagName = params.get('dagName') || '';
  }
  if (params.has('automataName')) {
    next.automataName = params.get('automataName') || '';
  }
  if (params.has('dagRunId')) {
    next.dagRunId = params.get('dagRunId') || '';
  }
  if (params.has('attemptId')) {
    next.attemptId = params.get('attemptId') || '';
  }

  const parsedFrom = parseDateFromUrl(params.get('fromDate'), tzOffsetInSec);
  if (params.has('fromDate')) {
    next.fromDate = parsedFrom;
  }
  const parsedTo = parseDateFromUrl(params.get('toDate'), tzOffsetInSec);
  if (params.has('toDate')) {
    next.toDate = parsedTo;
  }

  const dateMode = params.get('dateMode');
  if (dateMode === 'preset' || dateMode === 'specific' || dateMode === 'custom') {
    next.dateRangeMode = dateMode;
  }

  const datePreset = params.get('preset');
  if (datePreset) {
    next.datePreset = datePreset;
  }

  const specificPeriod = params.get('specificPeriod');
  if (
    specificPeriod === 'date' ||
    specificPeriod === 'month' ||
    specificPeriod === 'year'
  ) {
    next.specificPeriod = specificPeriod;
  }

  const specificValue = params.get('specificValue');
  if (specificValue) {
    next.specificValue = specificValue;
  }

  return sanitizeFilters(next);
}

export function buildEventLogQuery(args: {
  filters: EventLogFilters;
  remoteKey: string;
  tzOffsetInSec?: number;
}): EventLogQueryParams {
  const { filters, remoteKey, tzOffsetInSec } = args;
  return {
    remoteNode: remoteKey,
    kind: filters.kind !== 'all' ? filters.kind : undefined,
    paginationMode: ComponentsParametersEventLogPaginationMode.cursor,
    type: filters.type !== 'all' ? filters.type : undefined,
    dagName: filters.dagName || undefined,
    automataName: filters.automataName || undefined,
    dagRunId: filters.dagRunId || undefined,
    attemptId: filters.attemptId || undefined,
    startTime: formatDateForApi(filters.fromDate, tzOffsetInSec),
    endTime: formatDateForApi(filters.toDate, tzOffsetInSec),
    limit: PAGE_SIZE,
  };
}
