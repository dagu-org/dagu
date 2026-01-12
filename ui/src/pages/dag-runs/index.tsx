import dayjs from 'dayjs';
import { ChevronDown, Layers, List, Search, Tag, X } from 'lucide-react';
import React from 'react';
import { useLocation } from 'react-router-dom';
import { Status } from '../../api/v2/schema';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { DateRangePicker } from '../../components/ui/date-range-picker';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { Input } from '../../components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../components/ui/select';
import { ToggleButton, ToggleGroup } from '../../components/ui/toggle-group';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import { useSearchState } from '../../contexts/SearchStateContext';
import { useUserPreferences } from '../../contexts/UserPreference';
import { DAGRunDetailsModal } from '../../features/dag-runs/components/dag-run-details';
import DAGRunGroupedView from '../../features/dag-runs/components/dag-run-list/DAGRunGroupedView';
import DAGRunTable from '../../features/dag-runs/components/dag-run-list/DAGRunTable';
import { useQuery } from '../../hooks/api';
import StatusChip from '../../ui/StatusChip';
import Title from '../../ui/Title';

type DAGRunsFilters = {
  searchText: string;
  dagRunId: string;
  status: string;
  tags: string[];
  fromDate?: string;
  toDate?: string;
  dateRangeMode: 'preset' | 'specific' | 'custom';
  datePreset: string;
  specificPeriod: 'date' | 'month' | 'year';
  specificValue: string;
};

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
  a.searchText === b.searchText &&
  a.dagRunId === b.dagRunId &&
  a.status === b.status &&
  areTagsEqual(a.tags, b.tags) &&
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

  // State for search input, dagRun ID, status, tags, and date ranges
  const [searchText, setSearchText] = React.useState(defaultFilters.searchText);
  const [dagRunId, setDagRunId] = React.useState(defaultFilters.dagRunId);
  const [status, setStatus] = React.useState<string>(defaultFilters.status);
  const [selectedTags, setSelectedTags] = React.useState<string[]>(
    defaultFilters.tags
  );
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
  const [apiDagRunId, setApiDagRunId] = React.useState(defaultFilters.dagRunId);
  const [apiStatus, setApiStatus] = React.useState(defaultFilters.status);
  const [apiTags, setApiTags] = React.useState<string[]>(defaultFilters.tags);
  const [apiFromDate, setApiFromDate] = React.useState<string | undefined>(
    defaultFilters.fromDate
  );
  const [apiToDate, setApiToDate] = React.useState<string | undefined>(
    defaultFilters.toDate
  );

  // State for selected DAG run in split layout
  const [selectedDAGRun, setSelectedDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);

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
      tags: selectedTags,
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
      selectedTags,
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

    if (params.has('tags')) {
      const tagsParam = params.get('tags') ?? '';
      urlFilters.tags = tagsParam
        ? tagsParam.split(',').map((t) => t.trim().toLowerCase()).filter((t) => t !== '')
        : [];
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
    if (
      dateModeParam === 'preset' ||
      dateModeParam === 'specific' ||
      dateModeParam === 'custom'
    ) {
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
      urlFilters.specificValue =
        params.get('specificValue') || defaultFilters.specificValue;
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
    setSelectedTags(next.tags);
    setFromDate(next.fromDate);
    setToDate(next.toDate);
    setDateRangeMode(next.dateRangeMode);
    setDatePreset(next.datePreset);
    setSpecificPeriod(next.specificPeriod);
    setSpecificValue(next.specificValue);

    setAPISearchText(next.searchText);
    setApiDagRunId(next.dagRunId);
    setApiStatus(next.status);
    setApiTags(next.tags);
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

  // Fetch available tags for the filter dropdown
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
          tags: apiTags.length > 0 ? apiTags.join(',') : undefined,
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
    setApiTags(selectedTags);
    setApiFromDate(fromDate);
    setApiToDate(toDate);

    // Update URL parameters
    addSearchParam('name', searchText);
    addSearchParam('dagRunId', dagRunId);
    addSearchParam('status', statusToUse);
    addSearchParam('tags', selectedTags.length > 0 ? selectedTags.join(',') : undefined);
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

  const updateTags = (newTags: string[]) => {
    setSelectedTags(newTags);
    setApiTags(newTags);
    addSearchParam('tags', newTags.length > 0 ? newTags.join(',') : undefined);
    mutate();
  };

  const handleTagToggle = (tag: string) => {
    const newTags = selectedTags.includes(tag)
      ? selectedTags.filter((t) => t !== tag)
      : [...selectedTags, tag];
    updateTags(newTags);
  };

  const handleClearTags = () => {
    updateTags([]);
  };

  const handleViewModeChange = (value: string) => {
    const newViewMode = value as 'list' | 'grouped';
    updatePreference('dagRunsViewMode', newViewMode);
  };

  const getPresetDates = (preset: string): { from: string; to?: string } => {
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

  const getSpecificPeriodDates = (
    period: 'date' | 'month' | 'year',
    value: string
  ): { from: string; to?: string } => {
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

  const handleSpecificPeriodChange = (
    value: string,
    period?: 'date' | 'month' | 'year'
  ) => {
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

  const handleDateRangeModeChange = (
    newMode: 'preset' | 'specific' | 'custom'
  ) => {
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
    <div className="">
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
            <Input
              placeholder="Filter by DAG name..."
              value={searchText}
              onChange={handleNameInputChange}
              onKeyDown={handleInputKeyPress}
              className="w-[220px]"
            />
            <Input
              placeholder="Filter by Run ID..."
              value={dagRunId}
              onChange={handleDagRunIdInputChange}
              onKeyDown={handleInputKeyPress}
              className="w-[200px]"
            />
            <Select value={status} onValueChange={handleStatusChange}>
              <SelectTrigger className="w-[160px]">
                <SelectValue placeholder="Status">
                  <StatusSelectDisplay status={status} />
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
                    <StatusChip status={Number(statusValue) as Status} size="sm">
                      {label}
                    </StatusChip>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {/* Tags filter */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" className="h-8 px-3">
                  <Tag size={14} className="mr-1.5" />
                  Tags
                  {selectedTags.length > 0 && (
                    <Badge
                      variant="secondary"
                      className="ml-1.5 h-5 px-1.5 text-xs"
                    >
                      {selectedTags.length}
                    </Badge>
                  )}
                  <ChevronDown size={14} className="ml-1.5 opacity-50" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-[200px]">
                <DropdownMenuLabel className="text-xs text-muted-foreground">
                  Filter by tags (AND)
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                {availableTags.length === 0 ? (
                  <div className="px-2 py-1.5 text-sm text-muted-foreground">
                    No tags available
                  </div>
                ) : (
                  availableTags.map((tag) => (
                    <DropdownMenuCheckboxItem
                      key={tag}
                      checked={selectedTags.includes(tag)}
                      onCheckedChange={() => handleTagToggle(tag)}
                      onSelect={(e) => e.preventDefault()}
                    >
                      {tag}
                    </DropdownMenuCheckboxItem>
                  ))
                )}
                {selectedTags.length > 0 && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      onSelect={() => handleClearTags()}
                    >
                      <X size={14} className="mr-1.5" />
                      Clear all
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
            {/* Selected tags display */}
            {selectedTags.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {selectedTags.map((tag) => (
                  <Badge
                    key={tag}
                    variant="secondary"
                    className="text-xs cursor-pointer hover:bg-destructive/20"
                    onClick={() => handleTagToggle(tag)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handleTagToggle(tag);
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`Remove tag ${tag}`}
                  >
                    {tag}
                    <X size={12} className="ml-1" />
                  </Badge>
                ))}
              </div>
            )}
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
              >
                Quick
              </ToggleButton>
              <ToggleButton
                value="specific"
                groupValue={dateRangeMode}
                onClick={() => handleDateRangeModeChange('specific')}
                position="middle"
                aria-label="Specific date/month/year"
              >
                Specific
              </ToggleButton>
              <ToggleButton
                value="custom"
                groupValue={dateRangeMode}
                onClick={() => handleDateRangeModeChange('custom')}
                position="last"
                aria-label="Custom range"
              >
                Custom
              </ToggleButton>
            </ToggleGroup>
            {dateRangeMode === 'preset' ? (
              <Select value={datePreset} onValueChange={handleDatePresetChange}>
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
            ) : dateRangeMode === 'specific' ? (
              <>
                <Select
                  value={specificPeriod}
                  onValueChange={(v) => {
                    const newPeriod = v as 'date' | 'month' | 'year';
                    setSpecificPeriod(newPeriod);
                    let newValue: string;
                    const parsedDate = dayjs(specificValue);

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

                    setSpecificValue(newValue);
                    handleSpecificPeriodChange(newValue, newPeriod);
                  }}
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
                  type={getInputTypeForPeriod(specificPeriod)}
                  value={specificValue}
                  onChange={(e) => handleSpecificPeriodChange(e.target.value)}
                  placeholder={specificPeriod === 'year' ? 'YYYY' : undefined}
                  min={specificPeriod === 'year' ? '2000' : undefined}
                  max={specificPeriod === 'year' ? '2100' : undefined}
                  className="w-[160px] h-8"
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
          <DAGRunTable
            dagRuns={data?.dagRuns || []}
            selectedDAGRun={selectedDAGRun}
            onSelectDAGRun={setSelectedDAGRun}
          />
        ) : (
          <DAGRunGroupedView dagRuns={data?.dagRuns || []} />
        )}
      </div>

      {/* Side Modal for DAG Run Details */}
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
