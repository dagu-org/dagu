import { components } from '@/api/v1/schema';

export type EventLogEntry = components['schemas']['EventLogEntry'];
export type EventLogsResponse = components['schemas']['EventLogsResponse'];

export type DateRangeMode = 'preset' | 'specific' | 'custom';
export type SpecificPeriod = 'date' | 'month' | 'year';
export type EventKindFilter = 'all' | 'dag_run' | 'automata' | 'llm_usage';

export type EventLogFilters = {
  kind: EventKindFilter;
  type: string;
  dagName: string;
  automataName: string;
  dagRunId: string;
  attemptId: string;
  fromDate?: string;
  toDate?: string;
  dateRangeMode: DateRangeMode;
  datePreset: string;
  specificPeriod: SpecificPeriod;
  specificValue: string;
};

export type StoredEventLogState = EventLogFilters;

export type EventLogQueryParams = {
  remoteNode: string;
  kind?: EventKindFilter;
  paginationMode: components['parameters']['EventLogPaginationMode'];
  type?: string;
  dagName?: string;
  automataName?: string;
  dagRunId?: string;
  attemptId?: string;
  startTime?: string;
  endTime?: string;
  limit: number;
};

export const PAGE_SIZE = 50;
export const DEFAULT_DATE_PRESET = 'last7days';
export const SEARCH_STATE_KEY = 'eventLogs';
