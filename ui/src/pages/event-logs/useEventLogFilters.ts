import { ComponentsParametersEventLogPaginationMode } from '@/api/v1/schema';
import dayjs from '@/lib/dayjs';
import * as React from 'react';
import {
  DEFAULT_DATE_PRESET,
  PAGE_SIZE,
  SEARCH_STATE_KEY,
  type DateRangeMode,
  type EventKindFilter,
  type EventLogFilters,
  type EventLogQueryParams,
  type SpecificPeriod,
  type StoredEventLogState,
} from './types';
import {
  getEventTypeOptions,
  isEventKindFilter,
  sanitizeEventTypeForKind,
} from './options';
import { areEventLogFiltersEqual, hasQueryParams } from './utils';

type SearchStateStore = {
  readState<T>(pageKey: string, remoteKey?: string): T | undefined;
  writeState<T>(pageKey: string, remoteKey: string | undefined, value: T): void;
};

type UseEventLogFiltersArgs = {
  tzOffsetInSec?: number;
  remoteKey: string;
  searchKey: string;
  locationSearchParams: URLSearchParams;
  searchState: SearchStateStore;
  setSearchParams: (next: URLSearchParams) => void;
};

function sanitizeFilters(filters: EventLogFilters): EventLogFilters {
  return {
    ...filters,
    type: sanitizeEventTypeForKind(filters.kind, filters.type),
  };
}

export function useEventLogFilters({
  tzOffsetInSec,
  remoteKey,
  searchKey,
  locationSearchParams,
  searchState,
  setSearchParams,
}: UseEventLogFiltersArgs) {
  const getPresetDates = React.useCallback(
    (preset: string): { from: string; to?: string } => {
      const now = dayjs();
      const startOfDay =
        tzOffsetInSec !== undefined
          ? now.utcOffset(tzOffsetInSec / 60).startOf('day')
          : now.startOf('day');

      switch (preset) {
        case 'today':
          return { from: startOfDay.format('YYYY-MM-DDTHH:mm:ss') };
        case 'yesterday':
          return {
            from: startOfDay.subtract(1, 'day').format('YYYY-MM-DDTHH:mm:ss'),
            to: startOfDay.format('YYYY-MM-DDTHH:mm:ss'),
          };
        case 'last30days':
          return {
            from: startOfDay.subtract(30, 'day').format('YYYY-MM-DDTHH:mm:ss'),
          };
        case 'thisWeek':
          return {
            from: startOfDay.startOf('week').format('YYYY-MM-DDTHH:mm:ss'),
          };
        case 'thisMonth':
          return {
            from: startOfDay.startOf('month').format('YYYY-MM-DDTHH:mm:ss'),
          };
        case 'last7days':
        default:
          return {
            from: startOfDay.subtract(7, 'day').format('YYYY-MM-DDTHH:mm:ss'),
          };
      }
    },
    [tzOffsetInSec]
  );

  const getSpecificPeriodDates = React.useCallback(
    (period: SpecificPeriod, value: string): { from: string; to?: string } => {
      const parsedDate = dayjs(value);
      if (!parsedDate.isValid()) {
        const fallback =
          tzOffsetInSec !== undefined
            ? dayjs().utcOffset(tzOffsetInSec / 60)
            : dayjs();
        return { from: fallback.startOf('day').format('YYYY-MM-DDTHH:mm:ss') };
      }

      const date =
        tzOffsetInSec !== undefined
          ? parsedDate.utcOffset(tzOffsetInSec / 60)
          : parsedDate;
      const unit = period === 'date' ? 'day' : period;
      return {
        from: date.startOf(unit).format('YYYY-MM-DDTHH:mm:ss'),
        to: date.endOf(unit).format('YYYY-MM-DDTHH:mm:ss'),
      };
    },
    [tzOffsetInSec]
  );

  const parseDateFromUrl = React.useCallback(
    (value: string | null) => {
      if (!value) {
        return undefined;
      }

      if (/^\d+$/.test(value)) {
        const timestamp = Number(value);
        if (!Number.isNaN(timestamp)) {
          const parsed =
            tzOffsetInSec !== undefined
              ? dayjs.unix(timestamp).utcOffset(tzOffsetInSec / 60)
              : dayjs.unix(timestamp);
          return parsed.format('YYYY-MM-DDTHH:mm:ss');
        }
      }

      if (value.includes('T') && value.length >= 16) {
        return value;
      }

      return undefined;
    },
    [tzOffsetInSec]
  );

  const formatDateForApi = React.useCallback(
    (dateString: string | undefined): string | undefined => {
      if (!dateString) {
        return undefined;
      }
      const dateWithSeconds =
        dateString.split(':').length < 3 ? `${dateString}:00` : dateString;
      if (tzOffsetInSec !== undefined) {
        return dayjs(dateWithSeconds)
          .utcOffset(tzOffsetInSec / 60, true)
          .toISOString();
      }
      return dayjs(dateWithSeconds).toISOString();
    },
    [tzOffsetInSec]
  );

  const formatTimestamp = React.useCallback(
    (timestamp: string): string => {
      const parsed =
        tzOffsetInSec !== undefined
          ? dayjs(timestamp).utcOffset(tzOffsetInSec / 60)
          : dayjs(timestamp);
      return parsed.format('YYYY-MM-DD HH:mm:ss');
    },
    [tzOffsetInSec]
  );

  const formatTimezoneOffset = React.useCallback((): string => {
    if (tzOffsetInSec === undefined) {
      return '';
    }
    const offsetInMinutes = tzOffsetInSec / 60;
    const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
    const minutes = Math.abs(offsetInMinutes) % 60;
    const sign = offsetInMinutes >= 0 ? '+' : '-';
    return `(${sign}${hours.toString().padStart(2, '0')}:${minutes
      .toString()
      .padStart(2, '0')})`;
  }, [tzOffsetInSec]);

  const defaultFilters = React.useMemo<EventLogFilters>(() => {
    const dates = getPresetDates(DEFAULT_DATE_PRESET);
    return {
      kind: 'all',
      type: 'all',
      dagName: '',
      automataName: '',
      dagRunId: '',
      attemptId: '',
      fromDate: dates.from,
      toDate: dates.to,
      dateRangeMode: 'preset',
      datePreset: DEFAULT_DATE_PRESET,
      specificPeriod: 'date',
      specificValue: dayjs().format('YYYY-MM-DD'),
    };
  }, [getPresetDates]);

  const [draftFilters, setDraftFilters] =
    React.useState<EventLogFilters>(defaultFilters);
  const [appliedFilters, setAppliedFilters] =
    React.useState<EventLogFilters>(defaultFilters);
  const [hydratedKey, setHydratedKey] = React.useState('');
  const lastPersistedAppliedFiltersRef = React.useRef<EventLogFilters | null>(
    null
  );

  const buildLocationParams = React.useCallback((filters: EventLogFilters) => {
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
  }, []);

  const parseLocationState = React.useCallback(
    (params: URLSearchParams): StoredEventLogState => {
      const persisted = searchState.readState<StoredEventLogState>(
        SEARCH_STATE_KEY,
        remoteKey
      );
      const base = sanitizeFilters({
        ...defaultFilters,
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

      const parsedFrom = parseDateFromUrl(params.get('fromDate'));
      if (params.has('fromDate')) {
        next.fromDate = parsedFrom;
      }
      const parsedTo = parseDateFromUrl(params.get('toDate'));
      if (params.has('toDate')) {
        next.toDate = parsedTo;
      }

      const dateMode = params.get('dateMode');
      if (
        dateMode === 'preset' ||
        dateMode === 'specific' ||
        dateMode === 'custom'
      ) {
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
    },
    [defaultFilters, parseDateFromUrl, remoteKey, searchState]
  );

  React.useEffect(() => {
    const next = parseLocationState(locationSearchParams);
    setDraftFilters(next);
    setAppliedFilters(next);
    lastPersistedAppliedFiltersRef.current = next;
    searchState.writeState(SEARCH_STATE_KEY, remoteKey, next);
    setHydratedKey(`${remoteKey}:${searchKey}`);
  }, [
    locationSearchParams,
    parseLocationState,
    remoteKey,
    searchKey,
    searchState,
  ]);

  React.useEffect(() => {
    if (hydratedKey !== `${remoteKey}:${searchKey}`) {
      return;
    }
    const nextAppliedFilters = sanitizeFilters(appliedFilters);
    const persisted = lastPersistedAppliedFiltersRef.current;
    if (
      persisted &&
      areEventLogFiltersEqual(persisted, nextAppliedFilters)
    ) {
      return;
    }
    lastPersistedAppliedFiltersRef.current = nextAppliedFilters;
    searchState.writeState(SEARCH_STATE_KEY, remoteKey, nextAppliedFilters);
  }, [appliedFilters, hydratedKey, remoteKey, searchKey, searchState]);

  const query = React.useMemo<EventLogQueryParams>(() => {
    return {
      remoteNode: remoteKey,
      kind: appliedFilters.kind !== 'all' ? appliedFilters.kind : undefined,
      paginationMode: ComponentsParametersEventLogPaginationMode.cursor,
      type: appliedFilters.type !== 'all' ? appliedFilters.type : undefined,
      dagName: appliedFilters.dagName || undefined,
      automataName: appliedFilters.automataName || undefined,
      dagRunId: appliedFilters.dagRunId || undefined,
      attemptId: appliedFilters.attemptId || undefined,
      startTime: formatDateForApi(appliedFilters.fromDate),
      endTime: formatDateForApi(appliedFilters.toDate),
      limit: PAGE_SIZE,
    };
  }, [appliedFilters, formatDateForApi, remoteKey]);

  const isReady = hydratedKey === `${remoteKey}:${searchKey}`;

  const updateDraftFilters = React.useCallback(
    (patch: Partial<EventLogFilters>) => {
      setDraftFilters((prev) => sanitizeFilters({ ...prev, ...patch }));
    },
    []
  );

  const handleKindChange = React.useCallback((value: string) => {
    const kind = isEventKindFilter(value) ? value : 'all';
    setDraftFilters((prev) =>
      sanitizeFilters({
        ...prev,
        kind,
        type: sanitizeEventTypeForKind(kind, prev.type),
      })
    );
  }, []);

  const handleTypeChange = React.useCallback((value: string) => {
    setDraftFilters((prev) => ({
      ...prev,
      type: sanitizeEventTypeForKind(prev.kind, value),
    }));
  }, []);

  const applyFilters = React.useCallback(
    (nextFilters: EventLogFilters) => {
      const sanitized = sanitizeFilters(nextFilters);
      setAppliedFilters(sanitized);
      setSearchParams(buildLocationParams(sanitized));
    },
    [buildLocationParams, setSearchParams]
  );

  const handleApplyFilters = React.useCallback(() => {
    applyFilters(draftFilters);
  }, [applyFilters, draftFilters]);

  const handleClearFilters = React.useCallback(() => {
    setDraftFilters(defaultFilters);
    setAppliedFilters(defaultFilters);
    setSearchParams(buildLocationParams(defaultFilters));
  }, [buildLocationParams, defaultFilters, setSearchParams]);

  const handleDatePresetChange = React.useCallback(
    (preset: string) => {
      const dates = getPresetDates(preset);
      updateDraftFilters({
        datePreset: preset,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [getPresetDates, updateDraftFilters]
  );

  const handleSpecificPeriodChange = React.useCallback(
    (value: string, period?: SpecificPeriod) => {
      const nextPeriod = period || draftFilters.specificPeriod;
      const dates = getSpecificPeriodDates(nextPeriod, value);
      updateDraftFilters({
        specificValue: value,
        specificPeriod: nextPeriod,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [draftFilters.specificPeriod, getSpecificPeriodDates, updateDraftFilters]
  );

  const handleDateRangeModeChange = React.useCallback(
    (nextMode: DateRangeMode) => {
      if (nextMode === 'preset') {
        const dates = getPresetDates(draftFilters.datePreset);
        updateDraftFilters({
          dateRangeMode: nextMode,
          fromDate: dates.from,
          toDate: dates.to,
        });
        return;
      }
      if (nextMode === 'specific') {
        const dates = getSpecificPeriodDates(
          draftFilters.specificPeriod,
          draftFilters.specificValue
        );
        updateDraftFilters({
          dateRangeMode: nextMode,
          fromDate: dates.from,
          toDate: dates.to,
        });
        return;
      }
      updateDraftFilters({ dateRangeMode: nextMode });
    },
    [
      draftFilters.datePreset,
      draftFilters.specificPeriod,
      draftFilters.specificValue,
      getPresetDates,
      getSpecificPeriodDates,
      updateDraftFilters,
    ]
  );

  const handleSpecificPeriodSelect = React.useCallback(
    (value: string) => {
      const nextPeriod = value as SpecificPeriod;
      const parsedDate = dayjs(draftFilters.specificValue);
      let nextValue: string;

      if (nextPeriod === 'date') {
        nextValue = parsedDate.isValid()
          ? parsedDate.format('YYYY-MM-DD')
          : dayjs().format('YYYY-MM-DD');
      } else if (nextPeriod === 'month') {
        nextValue = parsedDate.isValid()
          ? parsedDate.format('YYYY-MM')
          : dayjs().format('YYYY-MM');
      } else {
        nextValue = parsedDate.isValid()
          ? parsedDate.format('YYYY')
          : dayjs().format('YYYY');
      }

      handleSpecificPeriodChange(nextValue, nextPeriod);
    },
    [draftFilters.specificValue, handleSpecificPeriodChange]
  );

  const eventTypeOptions = React.useMemo(
    () => getEventTypeOptions(draftFilters.kind),
    [draftFilters.kind]
  );

  return {
    draftFilters,
    appliedFilters,
    query,
    isReady,
    eventTypeOptions,
    updateDraftFilters,
    handleKindChange,
    handleTypeChange,
    handleApplyFilters,
    handleClearFilters,
    handleDatePresetChange,
    handleSpecificPeriodChange,
    handleDateRangeModeChange,
    handleSpecificPeriodSelect,
    formatTimestamp,
    formatTimezoneOffset,
  };
}
