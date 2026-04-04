import { Layers, List, Search } from 'lucide-react';
import React from 'react';
import { useSearchParams } from 'react-router-dom';
import { Status } from '../../api/v1/schema';
import { AutocompleteInput } from '../../components/ui/autocomplete-input';
import { Button } from '../../components/ui/button';
import { DateRangePicker } from '../../components/ui/date-range-picker';
import { Input } from '../../components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../components/ui/select';
import { TagCombobox } from '../../components/ui/tag-combobox';
import { ToggleButton, ToggleGroup } from '../../components/ui/toggle-group';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import DAGRunBatchActions from '../../features/dag-runs/components/common/DAGRunBatchActions';
import { DAGRunDetailsModal } from '../../features/dag-runs/components/dag-run-details';
import DAGRunGroupedView from '../../features/dag-runs/components/dag-run-list/DAGRunGroupedView';
import DAGRunTable from '../../features/dag-runs/components/dag-run-list/DAGRunTable';
import { usePaginatedDAGRuns } from '../../features/dag-runs/hooks/dagRunPagination';
import { useDAGRunFilterSuggestions } from '../../features/dag-runs/hooks/useDAGRunFilterSuggestions';
import { useBulkDAGRunSelection } from '../../features/dag-runs/hooks/useBulkDAGRunSelection';
import { useQuery } from '../../hooks/api';
import dayjs from '../../lib/dayjs';
import StatusChip from '../../ui/StatusChip';
import Title from '../../ui/Title';

type DateRangeMode = 'preset' | 'specific' | 'custom';
type SpecificPeriod = 'date' | 'month' | 'year';

type DAGRunsFilters = {
  name: string;
  dagRunId: string;
  status: string;
  tags: string[];
  fromDate?: string;
  toDate?: string;
  dateRangeMode: DateRangeMode;
  datePreset: string;
  specificPeriod: SpecificPeriod;
  specificValue: string;
};

const SEARCH_STATE_KEY = 'dagRuns';

const areTagsEqual = (a: string[], b: string[]): boolean => {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((tag, i) => tag === sortedB[i]);
};

const STATUS_CONFIG: Record<Status, string> = {
  [Status.NotStarted]: 'not_started',
  [Status.Running]: 'running',
  [Status.Failed]: 'failed',
  [Status.Aborted]: 'aborted',
  [Status.Success]: 'succeeded',
  [Status.Queued]: 'queued',
  [Status.PartialSuccess]: 'partially_succeeded',
  [Status.Waiting]: 'waiting',
  [Status.Rejected]: 'rejected',
};

function StatusSelectDisplay({ status }: { status: string }): React.ReactNode {
  if (status === 'all') {
    return (
      <div className="inline-flex items-center rounded-full border bg-muted border-border text-foreground py-0.5 px-2 text-xs font-medium">
        All
      </div>
    );
  }

  const statusNum = parseInt(status) as Status;
  const label = STATUS_CONFIG[statusNum];
  if (label) {
    return (
      <StatusChip status={statusNum} size="sm">
        {label}
      </StatusChip>
    );
  }

  return null;
}

const areFiltersEqual = (a: DAGRunsFilters, b: DAGRunsFilters): boolean =>
  a.name === b.name &&
  a.dagRunId === b.dagRunId &&
  a.status === b.status &&
  areTagsEqual(a.tags, b.tags) &&
  a.fromDate === b.fromDate &&
  a.toDate === b.toDate &&
  a.dateRangeMode === b.dateRangeMode &&
  a.datePreset === b.datePreset &&
  a.specificPeriod === b.specificPeriod &&
  a.specificValue === b.specificValue;

function hasQueryParams(params: URLSearchParams): boolean {
  return Array.from(params.keys()).length > 0;
}

function DAGRuns() {
  const [searchParams, setSearchParams] = useSearchParams();
  const searchKey = searchParams.toString();
  const locationSearchParams = React.useMemo(
    () => new URLSearchParams(searchKey),
    [searchKey]
  );
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const { preferences, updatePreference } = useUserPreferences();
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  const parseDateFromUrl = React.useCallback(
    (dateParam: string | null): string | undefined => {
      if (!dateParam) return undefined;

      if (/^\d+$/.test(dateParam)) {
        const timestamp = Number(dateParam);
        if (!Number.isNaN(timestamp)) {
          const parsed =
            config.tzOffsetInSec !== undefined
              ? dayjs.unix(timestamp).utcOffset(config.tzOffsetInSec / 60)
              : dayjs.unix(timestamp);
          return parsed.format('YYYY-MM-DDTHH:mm');
        }
      }

      const match = dateParam.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})/);
      if (match) {
        return match[1];
      }

      if (dateParam.includes('T') && dateParam.length >= 16) {
        return dateParam.slice(0, 16);
      }

      return undefined;
    },
    [config.tzOffsetInSec]
  );

  const formatDateForApi = React.useCallback(
    (dateString: string | undefined): number | undefined => {
      if (!dateString) return undefined;

      const dateWithSeconds =
        dateString.split(':').length < 3 ? `${dateString}:00` : dateString;

      if (config.tzOffsetInSec !== undefined) {
        return dayjs(dateWithSeconds)
          .utcOffset(config.tzOffsetInSec / 60)
          .unix();
      }

      return dayjs(dateWithSeconds).unix();
    },
    [config.tzOffsetInSec]
  );

  const getDefaultFromDate = React.useCallback((): string => {
    const now = dayjs();
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');
    return startOfDay.format('YYYY-MM-DDTHH:mm');
  }, [config.tzOffsetInSec]);

  const defaultFilters = React.useMemo<DAGRunsFilters>(
    () => ({
      name: '',
      dagRunId: '',
      status: 'all',
      tags: [],
      fromDate: getDefaultFromDate(),
      toDate: undefined,
      dateRangeMode: 'preset',
      datePreset: 'today',
      specificPeriod: 'date',
      specificValue: dayjs().format('YYYY-MM-DD'),
    }),
    [getDefaultFromDate]
  );

  const [draftFilters, setDraftFilters] =
    React.useState<DAGRunsFilters>(defaultFilters);
  const [appliedFilters, setAppliedFilters] =
    React.useState<DAGRunsFilters>(defaultFilters);
  const draftFiltersRef = React.useRef(draftFilters);
  const appliedFiltersRef = React.useRef(appliedFilters);
  const [hydratedKey, setHydratedKey] = React.useState('');
  const lastPersistedAppliedFiltersRef = React.useRef<DAGRunsFilters | null>(
    null
  );
  const pendingLocationSyncRef = React.useRef<string | null>(null);
  const [selectedDAGRun, setSelectedDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);
  const [isNameAutocompleteOpen, setIsNameAutocompleteOpen] =
    React.useState(false);
  const [isDagRunIdAutocompleteOpen, setIsDagRunIdAutocompleteOpen] =
    React.useState(false);

  React.useEffect(() => {
    draftFiltersRef.current = draftFilters;
  }, [draftFilters]);

  React.useEffect(() => {
    appliedFiltersRef.current = appliedFilters;
  }, [appliedFilters]);

  const viewMode = preferences.dagRunsViewMode;

  const buildLocationParams = React.useCallback((filters: DAGRunsFilters) => {
    const params = new URLSearchParams();

    if (filters.name) {
      params.set('name', filters.name);
    }
    if (filters.dagRunId) {
      params.set('dagRunId', filters.dagRunId);
    }
    if (filters.status && filters.status !== 'all') {
      params.set('status', filters.status);
    }
    if (filters.tags.length > 0) {
      params.set('tags', filters.tags.join(','));
    }
    if (filters.fromDate) {
      params.set('fromDate', filters.fromDate);
    }
    if (filters.toDate) {
      params.set('toDate', filters.toDate);
    }

    params.set('dateMode', filters.dateRangeMode);
    params.set('preset', filters.datePreset);
    params.set('specificValue', filters.specificValue);
    params.set('specificPeriod', filters.specificPeriod);

    return params;
  }, []);

  const parseLocationState = React.useCallback(
    (params: URLSearchParams): DAGRunsFilters => {
      const persisted = searchState.readState<DAGRunsFilters>(
        SEARCH_STATE_KEY,
        remoteKey
      );
      const base: DAGRunsFilters = {
        ...defaultFilters,
        ...(persisted ?? {}),
      };

      if (!hasQueryParams(params)) {
        return base;
      }

      const next: DAGRunsFilters = { ...base };

      if (params.has('name')) {
        next.name = params.get('name') ?? '';
      }

      if (params.has('dagRunId')) {
        next.dagRunId = params.get('dagRunId') ?? '';
      }

      if (params.has('status')) {
        next.status = params.get('status') || 'all';
      }

      if (params.has('tags')) {
        const tagsParam = params.get('tags') ?? '';
        next.tags = tagsParam
          ? tagsParam
              .split(',')
              .map((tag) => tag.trim().toLowerCase())
              .filter((tag) => tag !== '')
          : [];
      }

      if (params.has('fromDate')) {
        next.fromDate = parseDateFromUrl(params.get('fromDate'));
      }

      if (params.has('toDate')) {
        next.toDate = parseDateFromUrl(params.get('toDate'));
      }

      const dateModeParam = params.get('dateMode');
      if (
        dateModeParam === 'preset' ||
        dateModeParam === 'specific' ||
        dateModeParam === 'custom'
      ) {
        next.dateRangeMode = dateModeParam;
      }

      if (params.has('preset')) {
        next.datePreset = params.get('preset') || defaultFilters.datePreset;
      }

      const specificPeriodParam = params.get('specificPeriod');
      if (
        specificPeriodParam === 'date' ||
        specificPeriodParam === 'month' ||
        specificPeriodParam === 'year'
      ) {
        next.specificPeriod = specificPeriodParam;
      }

      if (params.has('specificValue')) {
        next.specificValue =
          params.get('specificValue') || defaultFilters.specificValue;
      }

      return next;
    },
    [defaultFilters, parseDateFromUrl, remoteKey, searchState]
  );

  React.useEffect(() => {
    appBarContext.setTitle('DAG Runs');
  }, [appBarContext]);

  React.useEffect(() => {
    if (pendingLocationSyncRef.current === searchKey) {
      pendingLocationSyncRef.current = null;
      setHydratedKey(`${remoteKey}:${searchKey}`);
      return;
    }

    const next = parseLocationState(locationSearchParams);
    draftFiltersRef.current = next;
    appliedFiltersRef.current = next;
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

    const persisted = lastPersistedAppliedFiltersRef.current;
    if (persisted && areFiltersEqual(persisted, appliedFilters)) {
      return;
    }

    lastPersistedAppliedFiltersRef.current = appliedFilters;
    searchState.writeState(SEARCH_STATE_KEY, remoteKey, appliedFilters);
  }, [appliedFilters, hydratedKey, remoteKey, searchKey, searchState]);

  const isReady = hydratedKey === `${remoteKey}:${searchKey}`;

  const updateDraftFilters = React.useCallback(
    (patch: Partial<DAGRunsFilters>) => {
      const next = {
        ...draftFiltersRef.current,
        ...patch,
      };
      draftFiltersRef.current = next;
      setDraftFilters(next);
    },
    []
  );

  const applyFilters = React.useCallback(
    (nextFilters: DAGRunsFilters) => {
      appliedFiltersRef.current = nextFilters;
      setAppliedFilters(nextFilters);
      const nextParams = buildLocationParams(nextFilters);
      pendingLocationSyncRef.current = nextParams.toString();
      setSearchParams(nextParams);
    },
    [buildLocationParams, setSearchParams]
  );

  const applyImmediateFilters = React.useCallback(
    (patch: Partial<DAGRunsFilters>) => {
      const nextDraft = {
        ...draftFiltersRef.current,
        ...patch,
      };
      const nextApplied = {
        ...appliedFiltersRef.current,
        ...patch,
      };

      draftFiltersRef.current = nextDraft;
      appliedFiltersRef.current = nextApplied;
      setDraftFilters(nextDraft);
      setAppliedFilters(nextApplied);
      const nextParams = buildLocationParams(nextApplied);
      pendingLocationSyncRef.current = nextParams.toString();
      setSearchParams(nextParams);
    },
    [buildLocationParams, setSearchParams]
  );

  const applyDraftOnlyFilters = React.useCallback(() => {
    const nextApplied = {
      ...appliedFiltersRef.current,
      name: draftFiltersRef.current.name,
      dagRunId: draftFiltersRef.current.dagRunId,
      fromDate: draftFiltersRef.current.fromDate,
      toDate: draftFiltersRef.current.toDate,
      dateRangeMode: draftFiltersRef.current.dateRangeMode,
      datePreset: draftFiltersRef.current.datePreset,
      specificPeriod: draftFiltersRef.current.specificPeriod,
      specificValue: draftFiltersRef.current.specificValue,
    };

    applyFilters(nextApplied);
  }, [applyFilters]);

  const { data: tagsData } = useQuery(
    '/dags/tags',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
    },
    {
      revalidateOnFocus: false,
      revalidateIfStale: false,
    }
  );
  const availableTags = tagsData?.tags ?? [];

  const dagRunQuery = React.useMemo(
    () => ({
      remoteNode: remoteKey,
      name: appliedFilters.name || undefined,
      dagRunId: appliedFilters.dagRunId || undefined,
      status:
        appliedFilters.status !== 'all'
          ? parseInt(appliedFilters.status, 10)
          : undefined,
      tags:
        appliedFilters.tags.length > 0
          ? appliedFilters.tags.join(',')
          : undefined,
      fromDate: formatDateForApi(appliedFilters.fromDate),
      toDate: formatDateForApi(appliedFilters.toDate),
      limit: 100,
    }),
    [appliedFilters, formatDateForApi, remoteKey]
  );
  const {
    dagRuns,
    isLoadingMore,
    loadMoreError,
    hasMore,
    refresh: refreshDagRuns,
    loadMore: handleLoadMore,
  } = usePaginatedDAGRuns({
    query: dagRunQuery,
    enabled: isReady,
  });

  const dagNameSuggestions = useDAGRunFilterSuggestions({
    field: 'name',
    filters: draftFilters,
    remoteNode: remoteKey,
    isOpen: isNameAutocompleteOpen,
    formatDateForApi,
  });
  const dagRunIdSuggestions = useDAGRunFilterSuggestions({
    field: 'dagRunId',
    filters: draftFilters,
    remoteNode: remoteKey,
    isOpen: isDagRunIdAutocompleteOpen,
    formatDateForApi,
  });

  const {
    clearSelection,
    replaceSelection,
    selectAllLoaded,
    selectedKeys,
    selectedRuns,
    toggleSelection,
  } = useBulkDAGRunSelection(dagRuns);

  const handleViewModeChange = (value: string) => {
    const newViewMode = value as 'list' | 'grouped';
    updatePreference('dagRunsViewMode', newViewMode);
  };

  const getPresetDates = React.useCallback(
    (preset: string): { from: string; to?: string } => {
      const now = dayjs();
      const startOfDay =
        config.tzOffsetInSec !== undefined
          ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
          : now.startOf('day');

      switch (preset) {
        case 'today':
          return { from: startOfDay.format('YYYY-MM-DDTHH:mm') };
        case 'yesterday':
          return {
            from: startOfDay.subtract(1, 'day').format('YYYY-MM-DDTHH:mm'),
            to: startOfDay.format('YYYY-MM-DDTHH:mm'),
          };
        case 'last7days':
          return {
            from: startOfDay.subtract(7, 'day').format('YYYY-MM-DDTHH:mm'),
          };
        case 'last30days':
          return {
            from: startOfDay.subtract(30, 'day').format('YYYY-MM-DDTHH:mm'),
          };
        case 'thisWeek':
          return {
            from: startOfDay.startOf('week').format('YYYY-MM-DDTHH:mm'),
          };
        case 'thisMonth':
          return {
            from: startOfDay.startOf('month').format('YYYY-MM-DDTHH:mm'),
          };
        default:
          return { from: startOfDay.format('YYYY-MM-DDTHH:mm') };
      }
    },
    [config.tzOffsetInSec]
  );

  const handleDatePresetChange = React.useCallback(
    (preset: string) => {
      const dates = getPresetDates(preset);
      applyImmediateFilters({
        dateRangeMode: 'preset',
        datePreset: preset,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [applyImmediateFilters, getPresetDates]
  );

  const getSpecificPeriodDates = React.useCallback(
    (period: SpecificPeriod, value: string): { from: string; to?: string } => {
      const parsedDate = dayjs(value);
      const date = parsedDate.isValid() ? parsedDate : dayjs();

      switch (period) {
        case 'date':
          return {
            from: date.startOf('day').format('YYYY-MM-DDTHH:mm'),
            to: date.endOf('day').format('YYYY-MM-DDTHH:mm'),
          };
        case 'month':
          return {
            from: date.startOf('month').format('YYYY-MM-DDTHH:mm'),
            to: date.endOf('month').format('YYYY-MM-DDTHH:mm'),
          };
        case 'year':
          return {
            from: date.startOf('year').format('YYYY-MM-DDTHH:mm'),
            to: date.endOf('year').format('YYYY-MM-DDTHH:mm'),
          };
      }
    },
    []
  );

  const getInputTypeForPeriod = (period: SpecificPeriod): string => {
    switch (period) {
      case 'date':
        return 'date';
      case 'month':
        return 'month';
      case 'year':
        return 'number';
    }
  };

  const handleSpecificPeriodChange = React.useCallback(
    (value: string, period?: SpecificPeriod) => {
      const periodToUse = period || draftFiltersRef.current.specificPeriod;
      const dates = getSpecificPeriodDates(periodToUse, value);
      applyImmediateFilters({
        dateRangeMode: 'specific',
        specificValue: value,
        specificPeriod: periodToUse,
        fromDate: dates.from,
        toDate: dates.to,
      });
    },
    [applyImmediateFilters, getSpecificPeriodDates]
  );

  const handleDateRangeModeChange = React.useCallback(
    (newMode: DateRangeMode) => {
      if (newMode === 'preset') {
        handleDatePresetChange(draftFiltersRef.current.datePreset);
        return;
      }

      if (newMode === 'specific') {
        handleSpecificPeriodChange(
          draftFiltersRef.current.specificValue,
          draftFiltersRef.current.specificPeriod
        );
        return;
      }

      applyImmediateFilters({ dateRangeMode: newMode });
    },
    [applyImmediateFilters, handleDatePresetChange, handleSpecificPeriodChange]
  );

  const handleSpecificPeriodSelect = React.useCallback(
    (value: string) => {
      const newPeriod = value as SpecificPeriod;
      const parsedDate = dayjs(draftFiltersRef.current.specificValue);
      let newValue: string;

      if (newPeriod === 'date') {
        newValue = parsedDate.isValid()
          ? parsedDate.format('YYYY-MM-DD')
          : dayjs().format('YYYY-MM-DD');
      } else if (newPeriod === 'month') {
        newValue = parsedDate.isValid()
          ? parsedDate.format('YYYY-MM')
          : dayjs().format('YYYY-MM');
      } else {
        newValue = parsedDate.isValid()
          ? parsedDate.format('YYYY')
          : dayjs().format('YYYY');
      }

      handleSpecificPeriodChange(newValue, newPeriod);
    },
    [handleSpecificPeriodChange]
  );

  const formatTimezoneOffset = (): string => {
    if (config.tzOffsetInSec === undefined) return '';

    const offsetInMinutes = config.tzOffsetInSec / 60;
    const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
    const minutes = Math.abs(offsetInMinutes) % 60;
    const sign = offsetInMinutes >= 0 ? '+' : '-';
    const formattedHours = hours.toString().padStart(2, '0');
    const formattedMinutes = minutes.toString().padStart(2, '0');

    return `(${sign}${formattedHours}:${formattedMinutes})`;
  };

  const tzLabel = formatTimezoneOffset();

  return (
    <div className="max-w-7xl">
      <div className="flex items-center justify-between mb-2">
        <Title>DAG Runs</Title>
        <ToggleGroup aria-label="View mode">
          <ToggleButton
            value="list"
            groupValue={viewMode}
            onClick={() => handleViewModeChange('list')}
            position="first"
            aria-label="List view"
            className="h-8 px-3"
          >
            <List size={16} className="mr-1.5" />
            List
          </ToggleButton>
          <ToggleButton
            value="grouped"
            groupValue={viewMode}
            onClick={() => handleViewModeChange('grouped')}
            position="last"
            aria-label="Grouped view"
            className="h-8 px-3"
          >
            <Layers size={16} className="mr-1.5" />
            Grouped
          </ToggleButton>
        </ToggleGroup>
      </div>
      <div>
        <div className="bg-muted/50 rounded-lg mb-2 space-y-2">
          <div className="flex flex-wrap gap-2">
            <AutocompleteInput
              placeholder="Filter by DAG name..."
              value={draftFilters.name}
              onValueChange={(value) => updateDraftFilters({ name: value })}
              onSubmit={applyDraftOnlyFilters}
              onOpenChange={setIsNameAutocompleteOpen}
              options={dagNameSuggestions.suggestions}
              loading={dagNameSuggestions.isLoading}
              emptyText={
                dagNameSuggestions.error
                  ? 'Failed to load DAG names.'
                  : 'No matching DAG names.'
              }
              className="w-[220px]"
            />
            <AutocompleteInput
              placeholder="Filter by Run ID..."
              value={draftFilters.dagRunId}
              onValueChange={(value) => updateDraftFilters({ dagRunId: value })}
              onSubmit={applyDraftOnlyFilters}
              onOpenChange={setIsDagRunIdAutocompleteOpen}
              options={dagRunIdSuggestions.suggestions}
              loading={dagRunIdSuggestions.isLoading}
              emptyText={
                dagRunIdSuggestions.error
                  ? 'Failed to load run IDs.'
                  : 'No matching run IDs.'
              }
              className="w-[200px]"
            />
            <Select
              value={draftFilters.status}
              onValueChange={(value) =>
                applyImmediateFilters({ status: value })
              }
            >
              <SelectTrigger className="w-[160px]">
                <SelectValue placeholder="Status">
                  <StatusSelectDisplay status={draftFilters.status} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  <div className="inline-flex items-center rounded-full border bg-muted border-border text-foreground py-0.5 px-2 text-xs font-medium">
                    All Statuses
                  </div>
                </SelectItem>
                {Object.entries(STATUS_CONFIG).map(([statusValue, label]) => (
                  <SelectItem key={statusValue} value={statusValue}>
                    <StatusChip
                      status={Number(statusValue) as Status}
                      size="sm"
                    >
                      {label}
                    </StatusChip>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <TagCombobox
              selectedTags={draftFilters.tags}
              onTagsChange={(tags) => applyImmediateFilters({ tags })}
              availableTags={availableTags}
              placeholder="Filter by tags..."
              className="min-w-[180px] max-w-[300px] h-7"
            />
            <Button
              onClick={applyDraftOnlyFilters}
              size="xs"
              className="px-6 font-medium"
            >
              <Search size={18} className="mr-2" />
              Search
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <ToggleGroup aria-label="Date range mode">
              <ToggleButton
                value="preset"
                groupValue={draftFilters.dateRangeMode}
                onClick={() => handleDateRangeModeChange('preset')}
                position="first"
                aria-label="Quick select"
              >
                Quick
              </ToggleButton>
              <ToggleButton
                value="specific"
                groupValue={draftFilters.dateRangeMode}
                onClick={() => handleDateRangeModeChange('specific')}
                position="middle"
                aria-label="Specific date/month/year"
              >
                Specific
              </ToggleButton>
              <ToggleButton
                value="custom"
                groupValue={draftFilters.dateRangeMode}
                onClick={() => handleDateRangeModeChange('custom')}
                position="last"
                aria-label="Custom range"
              >
                Custom
              </ToggleButton>
            </ToggleGroup>
            {draftFilters.dateRangeMode === 'preset' ? (
              <Select
                value={draftFilters.datePreset}
                onValueChange={handleDatePresetChange}
              >
                <SelectTrigger className="w-[180px]">
                  <SelectValue placeholder="Select period" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="today">Today</SelectItem>
                  <SelectItem value="yesterday">Yesterday</SelectItem>
                  <SelectItem value="last7days">Last 7 days</SelectItem>
                  <SelectItem value="last30days">Last 30 days</SelectItem>
                  <SelectItem value="thisWeek">This week</SelectItem>
                  <SelectItem value="thisMonth">This month</SelectItem>
                </SelectContent>
              </Select>
            ) : draftFilters.dateRangeMode === 'specific' ? (
              <>
                <Select
                  value={draftFilters.specificPeriod}
                  onValueChange={handleSpecificPeriodSelect}
                >
                  <SelectTrigger className="w-[120px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="date">Date</SelectItem>
                    <SelectItem value="month">Month</SelectItem>
                    <SelectItem value="year">Year</SelectItem>
                  </SelectContent>
                </Select>
                <Input
                  type={getInputTypeForPeriod(draftFilters.specificPeriod)}
                  value={draftFilters.specificValue}
                  onChange={(event) =>
                    handleSpecificPeriodChange(event.target.value)
                  }
                  placeholder={
                    draftFilters.specificPeriod === 'year' ? 'YYYY' : undefined
                  }
                  min={
                    draftFilters.specificPeriod === 'year' ? '2000' : undefined
                  }
                  max={
                    draftFilters.specificPeriod === 'year' ? '2100' : undefined
                  }
                  className="w-[160px] h-8"
                />
              </>
            ) : (
              <DateRangePicker
                fromDate={draftFilters.fromDate}
                toDate={draftFilters.toDate}
                onFromDateChange={(value) =>
                  updateDraftFilters({ fromDate: value })
                }
                onToDateChange={(value) =>
                  updateDraftFilters({ toDate: value })
                }
                onEnterPress={applyDraftOnlyFilters}
                fromLabel={`From ${tzLabel}`}
                toLabel={`To ${tzLabel}`}
                className="w-full md:w-auto"
              />
            )}
          </div>
        </div>
        <DAGRunBatchActions
          selectedRuns={selectedRuns}
          loadedCount={dagRuns.length}
          onSelectAllLoaded={selectAllLoaded}
          onClearSelection={clearSelection}
          onReplaceSelection={replaceSelection}
          onActionComplete={refreshDagRuns}
        />
        {viewMode === 'list' ? (
          <DAGRunTable
            dagRuns={dagRuns}
            selectedDAGRun={selectedDAGRun}
            onSelectDAGRun={setSelectedDAGRun}
            selectedRunKeys={selectedKeys}
            onToggleBulkSelect={toggleSelection}
          />
        ) : (
          <DAGRunGroupedView
            dagRuns={dagRuns}
            selectedDAGRun={selectedDAGRun}
            onSelectDAGRun={setSelectedDAGRun}
            selectedRunKeys={selectedKeys}
            onToggleBulkSelect={toggleSelection}
          />
        )}
        <div className="mt-3 flex flex-col items-center gap-2">
          {loadMoreError && (
            <div className="text-sm text-error">{loadMoreError}</div>
          )}
          {hasMore ? (
            <Button
              variant="outline"
              onClick={() => void handleLoadMore()}
              disabled={isLoadingMore}
            >
              {isLoadingMore ? 'Loading...' : 'Load more'}
            </Button>
          ) : dagRuns.length > 0 ? (
            <div className="text-sm text-muted-foreground">
              All loaded DAG runs are displayed.
            </div>
          ) : null}
        </div>
      </div>

      {selectedDAGRun && (
        <DAGRunDetailsModal
          name={selectedDAGRun.name}
          dagRunId={selectedDAGRun.dagRunId}
          isOpen={!!selectedDAGRun}
          onClose={() => setSelectedDAGRun(null)}
        />
      )}
    </div>
  );
}

export default DAGRuns;
