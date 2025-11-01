import dayjs from 'dayjs';
import { List, Layers, Search } from 'lucide-react';
import React from 'react';
import { useLocation } from 'react-router-dom';
import { Status } from '../../api/v2/schema';
import { Button } from '../../components/ui/button';
import { RefreshButton } from '../../components/ui/refresh-button';
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
import DAGRunTable from '../../features/dag-runs/components/dag-run-list/DAGRunTable';
import DAGRunGroupedView from '../../features/dag-runs/components/dag-run-list/DAGRunGroupedView';
import { useQuery } from '../../hooks/api';
import StatusChip from '../../ui/StatusChip';
import Title from '../../ui/Title';

function DAGRuns() {
  const query = new URLSearchParams(useLocation().search);
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();

  // Extract short datetime format from URL if present
  const parseDateFromUrl = (dateParam: string | null): string | undefined => {
    if (!dateParam) return undefined;
    // For datetime-local input, we need the format YYYY-MM-DDTHH:mm
    const match = dateParam.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})/);
    return match ? match[1] : undefined;
  };

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

  // State for search input, dagRun ID, status, and date ranges
  const [searchText, setSearchText] = React.useState(query.get('name') || '');
  const [dagRunId, setDagRunId] = React.useState(query.get('dagRunId') || '');
  const [status, setStatus] = React.useState<string>(
    query.get('status') || 'all'
  );

  // Default "From" date to the start of current day in the configured timezone
  const getDefaultFromDate = (): string => {
    const now = dayjs();
    // Apply timezone offset and set to beginning of day (00:00)
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');
    // Format for datetime-local input (YYYY-MM-DDTHH:mm)
    return startOfDay.format('YYYY-MM-DDTHH:mm');
  };

  const [fromDate, setFromDate] = React.useState<string | undefined>(
    parseDateFromUrl(query.get('fromDate')) || getDefaultFromDate()
  );
  const [toDate, setToDate] = React.useState<string | undefined>(
    parseDateFromUrl(query.get('toDate'))
  );

  // State for API parameters - these will be formatted with timezone
  const [apiSearchText, setAPISearchText] = React.useState(
    query.get('name') || ''
  );
  const [apiDagRunId, setApiDagRunId] = React.useState(
    query.get('dagRunId') || ''
  );
  const [apiStatus, setApiStatus] = React.useState(
    query.get('status') || 'all'
  );
  const [apiFromDate, setApiFromDate] = React.useState<string | undefined>(
    query.get('fromDate') || getDefaultFromDate()
  );
  const [apiToDate, setApiToDate] = React.useState<string | undefined>(
    query.get('toDate') || undefined
  );

  // State for view mode
  const [viewMode, setViewMode] = React.useState<'list' | 'grouped'>(
    (query.get('view') as 'list' | 'grouped') || 'list'
  );

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

  const addSearchParam = (key: string, value: string) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (value) {
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
    // Format dates for API and URL
    const timestampFromDate = formatDateForApi(fromDate);
    const timestampToDate = formatDateForApi(toDate);

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
    addSearchParam(
      'fromDate',
      timestampFromDate ? timestampFromDate.toString() : ''
    );
    addSearchParam('toDate', timestampToDate ? timestampToDate.toString() : '');

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
    setViewMode(newViewMode);
    addSearchParam('view', newViewMode);
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
            className="flex-1 min-w-[200px] bg-background"
          />
          <Input
            placeholder="Filter by Run ID..."
            value={dagRunId}
            onChange={handleDagRunIdInputChange}
            onKeyDown={handleInputKeyPress}
            className="flex-1 min-w-[180px] bg-background"
          />
          <Select value={status} onValueChange={handleStatusChange}>
            <SelectTrigger className="w-auto min-w-[140px] bg-background">
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
          <Button onClick={() => handleSearch()} size="sm" className="px-4">
            <Search size={16} className="mr-1" />
            Search
          </Button>
          <RefreshButton onRefresh={async () => { await mutate(); }} />
        </div>
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
