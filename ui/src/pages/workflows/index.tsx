import dayjs from 'dayjs';
import { Search } from 'lucide-react';
import React from 'react';
import { useLocation } from 'react-router-dom';
import { Button } from '../../components/ui/button';
import { DateRangePicker } from '../../components/ui/date-range-picker';
import { Input } from '../../components/ui/input';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useConfig } from '../../contexts/ConfigContext';
import WorkflowTable from '../../features/workflows/components/workflow-list/WorkflowTable';
import { useQuery } from '../../hooks/api';
import LoadingIndicator from '../../ui/LoadingIndicator';
import Title from '../../ui/Title';

function Workflows() {
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

  // State for search input, workflow ID, and date ranges
  const [searchText, setSearchText] = React.useState(query.get('name') || '');
  const [workflowId, setWorkflowId] = React.useState(
    query.get('workflowId') || ''
  );
  const [fromDate, setFromDate] = React.useState<string | undefined>(
    parseDateFromUrl(query.get('fromDate'))
  );
  const [toDate, setToDate] = React.useState<string | undefined>(
    parseDateFromUrl(query.get('toDate'))
  );

  // State for API parameters - these will be formatted with timezone
  const [apiSearchText, setAPISearchText] = React.useState(
    query.get('name') || ''
  );
  const [apiWorkflowId, setApiWorkflowId] = React.useState(
    query.get('workflowId') || ''
  );
  const [apiFromDate, setApiFromDate] = React.useState<string | undefined>(
    query.get('fromDate') || undefined
  );
  const [apiToDate, setApiToDate] = React.useState<string | undefined>(
    query.get('toDate') || undefined
  );

  React.useEffect(() => {
    appBarContext.setTitle('Workflows');
  }, [appBarContext]);

  const { data, isLoading } = useQuery(
    '/workflows',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          name: apiSearchText ? apiSearchText : undefined,
          workflowId: apiWorkflowId ? apiWorkflowId : undefined,
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

  const handleSearch = () => {
    // Format dates for API and URL
    const timestampFromDate = formatDateForApi(fromDate);
    const timestampToDate = formatDateForApi(toDate);

    // Console log for debugging
    console.log('Search with parameters:', {
      name: searchText,
      workflowId: workflowId,
      from: fromDate,
      to: toDate,
      timestampFrom: timestampFromDate,
      timestampTo: timestampToDate,
      tzOffset: config.tzOffsetInSec,
    });

    // Update API state with values
    setAPISearchText(searchText);
    setApiWorkflowId(workflowId);
    setApiFromDate(fromDate);
    setApiToDate(toDate);

    // Update URL parameters
    addSearchParam('name', searchText);
    addSearchParam('workflowId', workflowId);
    addSearchParam(
      'fromDate',
      timestampFromDate ? timestampFromDate.toString() : ''
    );
    addSearchParam('toDate', timestampToDate ? timestampToDate.toString() : '');
  };

  const handleNameInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  const handleWorkflowIdInputChange = (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setWorkflowId(e.target.value);
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
      <Title>Workflows</Title>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2 mb-4">
        <Input
          placeholder="Filter by workflow name..."
          value={searchText}
          onChange={handleNameInputChange}
          onKeyPress={handleInputKeyPress}
        />
        <Input
          placeholder="Filter by workflow ID..."
          value={workflowId}
          onChange={handleWorkflowIdInputChange}
          onKeyPress={handleInputKeyPress}
        />
        <div className="flex items-center justify-start">
          <Button onClick={handleSearch} className="w-full sm:w-auto">
            <Search size={18} className="mr-2" />
            Search
          </Button>
        </div>
        <div className="col-span-1 md:col-span-2 lg:col-span-3">
          <DateRangePicker
            fromDate={fromDate}
            toDate={toDate}
            onFromDateChange={setFromDate}
            onToDateChange={setToDate}
            fromLabel={`From ${tzLabel}`}
            toLabel={`To ${tzLabel}`}
            className="w-full md:w-auto md:min-w-[340px] md:max-w-[500px]"
          />
        </div>
      </div>
      {isLoading ? (
        <LoadingIndicator />
      ) : (
        <WorkflowTable workflows={data?.workflows || []} />
      )}
    </div>
  );
}

export default Workflows;
