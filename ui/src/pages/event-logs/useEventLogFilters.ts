import dayjs from '@/lib/dayjs';
import * as React from 'react';
import {
  createDefaultEventLogFilters,
  formatTimestamp,
  formatTimezoneOffset,
  getPresetDates,
  getSpecificPeriodDates,
} from './date_range';
import {
  buildEventLogQuery,
  buildLocationParams,
  parseLocationState,
  sanitizeFilters,
  type SearchStateStore,
} from './filter_state';
import {
  getEventTypeOptions,
  isEventKindFilter,
  sanitizeEventTypeForKind,
} from './options';
import {
  type DateRangeMode,
  type EventLogFilters,
  SEARCH_STATE_KEY,
  type SpecificPeriod,
} from './types';
import { areEventLogFiltersEqual } from './utils';

type UseEventLogFiltersArgs = {
  tzOffsetInSec?: number;
  remoteKey: string;
  searchKey: string;
  locationSearchParams: URLSearchParams;
  searchState: SearchStateStore;
  setSearchParams: (next: URLSearchParams) => void;
};

export function useEventLogFilters({
  tzOffsetInSec,
  remoteKey,
  searchKey,
  locationSearchParams,
  searchState,
  setSearchParams,
}: UseEventLogFiltersArgs) {
  const defaultFilters = React.useMemo<EventLogFilters>(
    () => createDefaultEventLogFilters(tzOffsetInSec),
    [tzOffsetInSec]
  );

  const [draftFilters, setDraftFilters] =
    React.useState<EventLogFilters>(defaultFilters);
  const [appliedFilters, setAppliedFilters] =
    React.useState<EventLogFilters>(defaultFilters);
  const [hydratedKey, setHydratedKey] = React.useState('');
  const lastPersistedAppliedFiltersRef = React.useRef<EventLogFilters | null>(
    null
  );

  React.useEffect(() => {
    const next = parseLocationState({
      params: locationSearchParams,
      remoteKey,
      searchState,
      tzOffsetInSec,
    });
    setDraftFilters(next);
    setAppliedFilters(next);
    lastPersistedAppliedFiltersRef.current = next;
    searchState.writeState(SEARCH_STATE_KEY, remoteKey, next);
    setHydratedKey(`${remoteKey}:${searchKey}`);
  }, [
    locationSearchParams,
    remoteKey,
    searchKey,
    searchState,
    tzOffsetInSec,
  ]);

  React.useEffect(() => {
    if (hydratedKey !== `${remoteKey}:${searchKey}`) {
      return;
    }
    const nextAppliedFilters = sanitizeFilters(appliedFilters);
    const persisted = lastPersistedAppliedFiltersRef.current;
    if (persisted && areEventLogFiltersEqual(persisted, nextAppliedFilters)) {
      return;
    }
    lastPersistedAppliedFiltersRef.current = nextAppliedFilters;
    searchState.writeState(SEARCH_STATE_KEY, remoteKey, nextAppliedFilters);
  }, [appliedFilters, hydratedKey, remoteKey, searchKey, searchState]);

  const query = React.useMemo(
    () =>
      buildEventLogQuery({
        filters: appliedFilters,
        remoteKey,
        tzOffsetInSec,
      }),
    [appliedFilters, remoteKey, tzOffsetInSec]
  );

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
    [setSearchParams]
  );

  const handleApplyFilters = React.useCallback(() => {
    applyFilters(draftFilters);
  }, [applyFilters, draftFilters]);

  const handleClearFilters = React.useCallback(() => {
    setDraftFilters(defaultFilters);
    setAppliedFilters(defaultFilters);
    setSearchParams(buildLocationParams(defaultFilters));
  }, [defaultFilters, setSearchParams]);

  const handleDatePresetChange = React.useCallback(
    (preset: string) => {
      const dates = getPresetDates(preset, tzOffsetInSec);
      updateDraftFilters({
        datePreset: preset,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [tzOffsetInSec, updateDraftFilters]
  );

  const handleSpecificPeriodChange = React.useCallback(
    (value: string, period?: SpecificPeriod) => {
      const nextPeriod = period || draftFilters.specificPeriod;
      const dates = getSpecificPeriodDates(nextPeriod, value, tzOffsetInSec);
      updateDraftFilters({
        specificValue: value,
        specificPeriod: nextPeriod,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [draftFilters.specificPeriod, tzOffsetInSec, updateDraftFilters]
  );

  const handleDateRangeModeChange = React.useCallback(
    (nextMode: DateRangeMode) => {
      if (nextMode === 'preset') {
        const dates = getPresetDates(draftFilters.datePreset, tzOffsetInSec);
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
          draftFilters.specificValue,
          tzOffsetInSec
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
      tzOffsetInSec,
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
    formatTimestamp: React.useCallback(
      (timestamp: string) => formatTimestamp(timestamp, tzOffsetInSec),
      [tzOffsetInSec]
    ),
    formatTimezoneOffset: React.useCallback(
      () => formatTimezoneOffset(tzOffsetInSec),
      [tzOffsetInSec]
    ),
  };
}
