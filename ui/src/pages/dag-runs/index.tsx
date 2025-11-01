import dayjs from 'dayjs';
import { List, Layers, Search } from 'lucide-react';
import React from 'react';
import { useLocation } from 'react-router-dom';
import { Status } from '../../api/v2/schema';
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
import { ToggleGroup, ToggleButton } from '../../components/ui/toggle-group';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import DAGRunTable from '../../features/dag-runs/components/dag-run-list/DAGRunTable';
import DAGRunGroupedView from '../../features/dag-runs/components/dag-run-list/DAGRunGroupedView';
import { useQuery } from '../../hooks/api';
import StatusChip from '../../ui/StatusChip';
import Title from '../../ui/Title';

type DAGRunsFilters = {
  searchText: string;
  dagRunId: string;
  status: string;
  fromDate?: string;
  toDate?: string;
  dateRangeMode: 'preset' | 'specific' | 'custom';
  datePreset: string;
  specificPeriod: 'date' | 'month' | 'year';
  specificValue: string;
};

const areFiltersEqual = (a: DAGRunsFilters, b: DAGRunsFilters) =>
  a.searchText === b.searchText &&
  a.dagRunId === b.dagRunId &&
  a.status === b.status &&
  a.fromDate === b.fromDate &&
  a.toDate === b.toDate &&
  a.dateRangeMode === b.dateRangeMode &&
  a.datePreset === b.datePreset &&
  a.specificPeriod === b.specificPeriod &&
  a.specificValue === b.specificValue;

function DAGRuns() {
  const location = useLocation();
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const { preferences, updatePreference } = useUserPreferences();
  const searchState = useSearchState();
  const remoteKey = appBarContext.selectedRemoteNode || 'local';

  // Extract short datetime format from URL if present
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

      // If the value already looks like a datetime-local string, normalize length
      if (dateParam.includes('T') && dateParam.length >= 16) {
        return dateParam.slice(0, 16);
      }

      return undefined;
    },
    [config.tzOffsetInSec]
  );
  // Convert datetime to unix timestamp (seconds) for API calls
  const formatDateForApi = (
    dateString: string | undefined
  ): number | undefined => {
    if (!dateString) return undefined;

    // Add seconds if they're missing (datetime-local inputs only have HH:mm)
    const dateWithSeconds =
      dateString.split(':').length < 3 ? `${dateString}:00` : dateString;

    // Apply timezone offset and convert to unix timestamp (seconds)
    if (config.tzOffsetInSec !== undefined) {
      return dayjs(dateWithSeconds)
        .utcOffset(config.tzOffsetInSec / 60)
        .unix();
    } else {
      return dayjs(dateWithSeconds).unix();
    }
  };

  // Default "From" date to the start of current day in the configured timezone
  const getDefaultFromDate = React.useCallback((): string => {
    const now = dayjs();
    // Apply timezone offset and set to beginning of day (00:00)
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');
    // Format for datetime-local input (YYYY-MM-DDTHH:mm)
    return startOfDay.format('YYYY-MM-DDTHH:mm');
  }, [config.tzOffsetInSec]);

  const defaultFilters = React.useMemo<DAGRunsFilters>(
    () => ({
      searchText: '',
      dagRunId: '',
      status: 'all',
      fromDate: getDefaultFromDate(),
      toDate: undefined,
      dateRangeMode: 'preset',
      datePreset: 'today',
      specificPeriod: 'date',
      specificValue: dayjs().format('YYYY-MM-DD'),
    }),
    [getDefaultFromDate]
  );

  // State for search input, dagRun ID, status, and date ranges
  const [searchText, setSearchText] = React.useState(defaultFilters.searchText);
  const [dagRunId, setDagRunId] = React.useState(defaultFilters.dagRunId);
  const [status, setStatus] = React.useState<string>(defaultFilters.status);
  const [fromDate, setFromDate] = React.useState<string | undefined>(
    defaultFilters.fromDate
  );
  const [toDate, setToDate] = React.useState<string | undefined>(
    defaultFilters.toDate
  );

  // State for API parameters - these will be formatted with timezone
  const [apiSearchText, setAPISearchText] = React.useState(
    defaultFilters.searchText
  );
  const [apiDagRunId, setApiDagRunId] = React.useState(
    defaultFilters.dagRunId
  );
  const [apiStatus, setApiStatus] = React.useState(defaultFilters.status);
  const [apiFromDate, setApiFromDate] = React.useState<string | undefined>(
    defaultFilters.fromDate
  );
  const [apiToDate, setApiToDate] = React.useState<string | undefined>(
    defaultFilters.toDate
  );

  // View mode comes from user preferences (local storage)
  const viewMode = preferences.dagRunsViewMode;

  // Date range mode: 'preset', 'specific', or 'custom'
  const [dateRangeMode, setDateRangeMode] = React.useState<
    'preset' | 'specific' | 'custom'
  >(defaultFilters.dateRangeMode);
  const [datePreset, setDatePreset] = React.useState<string>(
    defaultFilters.datePreset
  );
  const [specificPeriod, setSpecificPeriod] = React.useState<
    'date' | 'month' | 'year'
  >(defaultFilters.specificPeriod);
  const [specificValue, setSpecificValue] = React.useState<string>(
    defaultFilters.specificValue
  );

  const currentFilters = React.useMemo<DAGRunsFilters>(
    () => ({
      searchText,
      dagRunId,
      status,
      fromDate,
      toDate,
      dateRangeMode,
      datePreset,
      specificPeriod,
      specificValue,
    }),
    [
      searchText,
      dagRunId,
      status,
      fromDate,
      toDate,
      dateRangeMode,
      datePreset,
      specificPeriod,
      specificValue,
    ]
  );

  const currentFiltersRef = React.useRef(currentFilters);
  React.useEffect(() => {
    currentFiltersRef.current = currentFilters;
  }, [currentFilters]);

  const lastPersistedFiltersRef = React.useRef<DAGRunsFilters | null>(null);

  React.useEffect(() => {
    const params = new URLSearchParams(location.search);
    const stored = searchState.readState<DAGRunsFilters>('dagRuns', remoteKey);
    const base: DAGRunsFilters = {
      ...defaultFilters,
      ...(stored ?? {}),
    };

    const urlFilters: Partial<DAGRunsFilters> = {};
    let hasUrlFilters = false;

    if (params.has('name')) {
      urlFilters.searchText = params.get('name') ?? '';
      hasUrlFilters = true;
    }

    if (params.has('dagRunId')) {
      urlFilters.dagRunId = params.get('dagRunId') ?? '';
      hasUrlFilters = true;
    }

    if (params.has('status')) {
      urlFilters.status = params.get('status') || 'all';
      hasUrlFilters = true;
    }

    if (params.has('fromDate')) {
      urlFilters.fromDate = parseDateFromUrl(params.get('fromDate'));
      hasUrlFilters = true;
    }

    if (params.has('toDate')) {
      urlFilters.toDate = parseDateFromUrl(params.get('toDate'));
      hasUrlFilters = true;
    }

    const dateModeParam = params.get('dateMode');
    if (dateModeParam === 'preset' || dateModeParam === 'specific' || dateModeParam === 'custom') {
      urlFilters.dateRangeMode = dateModeParam;
      hasUrlFilters = true;
    }

    if (params.has('preset')) {
      urlFilters.datePreset = params.get('preset') || 'today';
      hasUrlFilters = true;
    }

    const specificPeriodParam = params.get('specificPeriod');
    if (
      specificPeriodParam === 'date' ||
      specificPeriodParam === 'month' ||
      specificPeriodParam === 'year'
    ) {
      urlFilters.specificPeriod = specificPeriodParam;
      hasUrlFilters = true;
    }

    if (params.has('specificValue')) {
      urlFilters.specificValue = params.get('specificValue') || defaultFilters.specificValue;
      hasUrlFilters = true;
    }

    const next = hasUrlFilters ? { ...base, ...urlFilters } : base;
    const current = currentFiltersRef.current;

    if (current && areFiltersEqual(current, next)) {
      if (hasUrlFilters) {
        lastPersistedFiltersRef.current = next;
        searchState.writeState('dagRuns', remoteKey, next);
      }
      return;
    }

    setSearchText(next.searchText);
    setDagRunId(next.dagRunId);
    setStatus(next.status);
    setFromDate(next.fromDate);
    setToDate(next.toDate);
    setDateRangeMode(next.dateRangeMode);
    setDatePreset(next.datePreset);
    setSpecificPeriod(next.specificPeriod);
    setSpecificValue(next.specificValue);

    setAPISearchText(next.searchText);
    setApiDagRunId(next.dagRunId);
    setApiStatus(next.status);
    setApiFromDate(next.fromDate);
    setApiToDate(next.toDate);

    lastPersistedFiltersRef.current = next;
    searchState.writeState('dagRuns', remoteKey, next);
  }, [
    defaultFilters,
    location.search,
    parseDateFromUrl,
    remoteKey,
    searchState,
  ]);

  React.useEffect(() => {
    const persisted = lastPersistedFiltersRef.current;
    if (persisted && areFiltersEqual(persisted, currentFilters)) {
      return;
    }
    lastPersistedFiltersRef.current = currentFilters;
    searchState.writeState('dagRuns', remoteKey, currentFilters);
  }, [currentFilters, remoteKey, searchState]);

  React.useEffect(() => {
    appBarContext.setTitle('DAG Runs');
  }, [appBarContext]);

  const { data, mutate } = useQuery(
    '/dag-runs',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          name: apiSearchText ? apiSearchText : undefined,
          dagRunId: apiDagRunId ? apiDagRunId : undefined,
          status:
            apiStatus && apiStatus !== 'all' ? parseInt(apiStatus) : undefined,
          fromDate: formatDateForApi(apiFromDate),
          toDate: formatDateForApi(apiToDate),
        },
      },
    },
    {
      // This ensures the query only runs when apiSearchText or date range changes
      revalidateIfStale: true,
      revalidateOnFocus: true,
      revalidateOnReconnect: true,
      refreshInterval: 1000,
    }
  );

  const addSearchParam = (key: string, value: string | undefined) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (value && value.length > 0) {
      locationQuery.set(key, value);
    } else {
      locationQuery.delete(key);
    }
    window.history.pushState(
      {},
      '',
      `${window.location.pathname}?${locationQuery.toString()}`
    );
  };

  const handleSearch = (overrideStatus?: string) => {
    // Use override status if provided, otherwise use current status
    const statusToUse = overrideStatus !== undefined ? overrideStatus : status;

    // Update API state with values
    setAPISearchText(searchText);
    setApiDagRunId(dagRunId);
    setApiStatus(statusToUse);
    setApiFromDate(fromDate);
    setApiToDate(toDate);

    // Update URL parameters
    addSearchParam('name', searchText);
    addSearchParam('dagRunId', dagRunId);
    addSearchParam('status', statusToUse);
    addSearchParam('fromDate', fromDate);
    addSearchParam('toDate', toDate);
    addSearchParam('dateMode', dateRangeMode);
    addSearchParam('preset', datePreset);
    addSearchParam('specificValue', specificValue);
    addSearchParam('specificPeriod', specificPeriod);

    // Force revalidation of the query even if parameters haven't changed
    mutate();
  };

  const handleNameInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  const handleDagRunIdInputChange = (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setDagRunId(e.target.value);
  };

  const handleStatusChange = (value: string) => {
    setStatus(value);
    // Automatically trigger search when status changes
    handleSearch(value);
  };

  const handleViewModeChange = (value: string) => {
    const newViewMode = value as 'list' | 'grouped';
    updatePreference('dagRunsViewMode', newViewMode);
  };

  const getPresetDates = (preset: string): { from: string; to?: string } => {
    const now = dayjs();
    const startOfDay = config.tzOffsetInSec !== undefined
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
        return { from: startOfDay.subtract(7, 'day').format('YYYY-MM-DDTHH:mm') };
      case 'last30days':
        return { from: startOfDay.subtract(30, 'day').format('YYYY-MM-DDTHH:mm') };
      case 'thisWeek':
        return { from: startOfDay.startOf('week').format('YYYY-MM-DDTHH:mm') };
      case 'thisMonth':
        return { from: startOfDay.startOf('month').format('YYYY-MM-DDTHH:mm') };
      default:
        return { from: startOfDay.format('YYYY-MM-DDTHH:mm') };
    }
  };

  const handleDatePresetChange = (preset: string) => {
    setDatePreset(preset);
    const dates = getPresetDates(preset);
    setFromDate(dates.from);
    setToDate(dates.to);
    setApiFromDate(dates.from);
    setApiToDate(dates.to);
    addSearchParam('preset', preset);
    addSearchParam('dateMode', 'preset');
    addSearchParam('fromDate', dates.from);
    addSearchParam('toDate', dates.to);
    // Trigger search with new dates
    mutate();
  };

  const getSpecificPeriodDates = (period: 'date' | 'month' | 'year', value: string): { from: string; to?: string } => {
    switch (period) {
      case 'date': {
        const date = dayjs(value);
        return {
          from: date.startOf('day').format('YYYY-MM-DDTHH:mm'),
          to: date.endOf('day').format('YYYY-MM-DDTHH:mm'),
        };
      }
      case 'month': {
        const date = dayjs(value);
        return {
          from: date.startOf('month').format('YYYY-MM-DDTHH:mm'),
          to: date.endOf('month').format('YYYY-MM-DDTHH:mm'),
        };
      }
      case 'year': {
        const date = dayjs(value);
        return {
          from: date.startOf('year').format('YYYY-MM-DDTHH:mm'),
          to: date.endOf('year').format('YYYY-MM-DDTHH:mm'),
        };
      }
    }
  };

  const getInputTypeForPeriod = (period: 'date' | 'month' | 'year'): string => {
    switch (period) {
      case 'date':
        return 'date';
      case 'month':
        return 'month';
      case 'year':
        return 'number';
    }
  };

  const handleSpecificPeriodChange = (value: string, period?: 'date' | 'month' | 'year') => {
    setSpecificValue(value);
    const periodToUse = period || specificPeriod;
    const dates = getSpecificPeriodDates(periodToUse, value);
    setFromDate(dates.from);
    setToDate(dates.to);
    setApiFromDate(dates.from);
    setApiToDate(dates.to);
    addSearchParam('specificValue', value);
    addSearchParam('specificPeriod', periodToUse);
    addSearchParam('dateMode', 'specific');
    addSearchParam('fromDate', dates.from);
    addSearchParam('toDate', dates.to);
    // Trigger search with new dates
    mutate();
  };

  const handleDateRangeModeChange = (newMode: 'preset' | 'specific' | 'custom') => {
    setDateRangeMode(newMode);
    addSearchParam('dateMode', newMode);

    if (newMode === 'preset') {
      // Apply current preset
      const dates = getPresetDates(datePreset);
      setFromDate(dates.from);
      setToDate(dates.to);
      setApiFromDate(dates.from);
      setApiToDate(dates.to);
      addSearchParam('preset', datePreset);
      addSearchParam('fromDate', dates.from);
      addSearchParam('toDate', dates.to);
      addSearchParam('specificValue', '');
      addSearchParam('specificPeriod', '');
      mutate();
    } else if (newMode === 'specific') {
      // Apply current specific period value
      const dates = getSpecificPeriodDates(specificPeriod, specificValue);
      setFromDate(dates.from);
      setToDate(dates.to);
      setApiFromDate(dates.from);
      setApiToDate(dates.to);
      addSearchParam('specificPeriod', specificPeriod);
      addSearchParam('specificValue', specificValue);
      addSearchParam('fromDate', dates.from);
      addSearchParam('toDate', dates.to);
      addSearchParam('preset', '');
      mutate();
    } else {
      addSearchParam('preset', '');
      addSearchParam('specificValue', '');
      addSearchParam('specificPeriod', '');
    }
  };

  const handleInputKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  // Format timezone offset for display
  const formatTimezoneOffset = (): string => {
    if (config.tzOffsetInSec === undefined) return '';

    // Convert seconds to hours and minutes
    const offsetInMinutes = config.tzOffsetInSec / 60;
    const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
    const minutes = Math.abs(offsetInMinutes) % 60;

    // Format with sign and padding
    const sign = offsetInMinutes >= 0 ? '+' : '-';
    const formattedHours = hours.toString().padStart(2, '0');
    const formattedMinutes = minutes.toString().padStart(2, '0');

    return `(${sign}${formattedHours}:${formattedMinutes})`;
  };

  const tzLabel = formatTimezoneOffset();

  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between mb-3">
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
      <div className="bg-muted/50 dark:bg-zinc-900/50 rounded-lg p-3 mb-4 space-y-3">
        <div className="flex flex-wrap gap-2">
          <Input
            placeholder="Filter by DAG name..."
            value={searchText}
            onChange={handleNameInputChange}
            onKeyDown={handleInputKeyPress}
            className="w-[220px] bg-background"
          />
          <Input
            placeholder="Filter by Run ID..."
            value={dagRunId}
            onChange={handleDagRunIdInputChange}
            onKeyDown={handleInputKeyPress}
            className="w-[200px] bg-background"
          />
          <Select value={status} onValueChange={handleStatusChange}>
            <SelectTrigger className="w-[160px] bg-background">
              <SelectValue placeholder="Status">
                {status === 'all' ? (
                  <div className="inline-flex items-center rounded-full border bg-gray-100 border-gray-300 text-gray-700 py-0.5 px-2 text-xs font-medium">
                    All
                  </div>
                ) : status === String(Status.NotStarted) ? (
                  <StatusChip status={Status.NotStarted} size="sm">
                    not_started
                  </StatusChip>
                ) : status === String(Status.Running) ? (
                  <StatusChip status={Status.Running} size="sm">
                    running
                  </StatusChip>
                ) : status === String(Status.Failed) ? (
                  <StatusChip status={Status.Failed} size="sm">
                    failed
                  </StatusChip>
                ) : status === String(Status.Cancelled) ? (
                  <StatusChip status={Status.Cancelled} size="sm">
                    canceled
                  </StatusChip>
                ) : status === String(Status.Success) ? (
                  <StatusChip status={Status.Success} size="sm">
                    succeeded
                  </StatusChip>
                ) : status === String(Status.Queued) ? (
                  <StatusChip status={Status.Queued} size="sm">
                    queued
                  </StatusChip>
                ) : status === String(Status.PartialSuccess) ? (
                  <StatusChip status={Status.PartialSuccess} size="sm">
                    partially_succeeded
                  </StatusChip>
                ) : null}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                <div className="inline-flex items-center rounded-full border bg-gray-100 border-gray-300 text-gray-700 py-0.5 px-2 text-xs font-medium">
                  All Statuses
                </div>
              </SelectItem>
              <SelectItem value={String(Status.NotStarted)}>
                <StatusChip status={Status.NotStarted} size="sm">
                  not_started
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.Running)}>
                <StatusChip status={Status.Running} size="sm">
                  running
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.Failed)}>
                <StatusChip status={Status.Failed} size="sm">
                  failed
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.Cancelled)}>
                <StatusChip status={Status.Cancelled} size="sm">
                  canceled
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.Success)}>
                <StatusChip status={Status.Success} size="sm">
                  succeeded
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.Queued)}>
                <StatusChip status={Status.Queued} size="sm">
                  queued
                </StatusChip>
              </SelectItem>
              <SelectItem value={String(Status.PartialSuccess)}>
                <StatusChip status={Status.PartialSuccess} size="sm">
                  partially_succeeded
                </StatusChip>
              </SelectItem>
            </SelectContent>
          </Select>
          <Button
            onClick={() => handleSearch()}
            size="default"
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
              groupValue={dateRangeMode}
              onClick={() => handleDateRangeModeChange('preset')}
              position="first"
              aria-label="Quick select"
              className="h-10 px-3 text-xs"
            >
              Quick
            </ToggleButton>
            <ToggleButton
              value="specific"
              groupValue={dateRangeMode}
              onClick={() => handleDateRangeModeChange('specific')}
              position="middle"
              aria-label="Specific date/month/year"
              className="h-10 px-3 text-xs"
            >
              Specific
            </ToggleButton>
            <ToggleButton
              value="custom"
              groupValue={dateRangeMode}
              onClick={() => handleDateRangeModeChange('custom')}
              position="last"
              aria-label="Custom range"
              className="h-10 px-3 text-xs"
            >
              Custom
            </ToggleButton>
          </ToggleGroup>
          {dateRangeMode === 'preset' ? (
            <Select value={datePreset} onValueChange={handleDatePresetChange}>
              <SelectTrigger className="w-[180px] bg-background">
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
          ) : dateRangeMode === 'specific' ? (
            <>
              <Select
                value={specificPeriod}
                onValueChange={(v) => {
                  const newPeriod = v as 'date' | 'month' | 'year';
                  setSpecificPeriod(newPeriod);
                  // Update the value format based on the new period type
                  // Extract the base date from current value and reformat
                  let newValue: string;
                  const parsedDate = dayjs(specificValue);

                  if (newPeriod === 'date') {
                    // If switching to date, use the parsed date or today
                    newValue = parsedDate.isValid() ? parsedDate.format('YYYY-MM-DD') : dayjs().format('YYYY-MM-DD');
                  } else if (newPeriod === 'month') {
                    // If switching to month, extract year-month from current value
                    newValue = parsedDate.isValid() ? parsedDate.format('YYYY-MM') : dayjs().format('YYYY-MM');
                  } else {
                    // If switching to year, extract year from current value
                    newValue = parsedDate.isValid() ? parsedDate.format('YYYY') : dayjs().format('YYYY');
                  }

                  setSpecificValue(newValue);
                  handleSpecificPeriodChange(newValue, newPeriod);
                }}
              >
                <SelectTrigger className="w-[120px] bg-background">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="date">Date</SelectItem>
                  <SelectItem value="month">Month</SelectItem>
                  <SelectItem value="year">Year</SelectItem>
                </SelectContent>
              </Select>
              <Input
                type={getInputTypeForPeriod(specificPeriod)}
                value={specificValue}
                onChange={(e) => handleSpecificPeriodChange(e.target.value)}
                placeholder={specificPeriod === 'year' ? 'YYYY' : undefined}
                min={specificPeriod === 'year' ? '2000' : undefined}
                max={specificPeriod === 'year' ? '2100' : undefined}
                className="w-[160px] bg-background h-10"
              />
            </>
          ) : (
            <DateRangePicker
              fromDate={fromDate}
              toDate={toDate}
              onFromDateChange={setFromDate}
              onToDateChange={setToDate}
              onEnterPress={() => handleSearch()}
              fromLabel={`From ${tzLabel}`}
              toLabel={`To ${tzLabel}`}
              className="w-full md:w-auto"
            />
          )}
        </div>
      </div>
      {viewMode === 'list' ? (
        <DAGRunTable dagRuns={data?.dagRuns || []} />
      ) : (
        <DAGRunGroupedView dagRuns={data?.dagRuns || []} />
      )}
    </div>
  );
}

export default DAGRuns;
