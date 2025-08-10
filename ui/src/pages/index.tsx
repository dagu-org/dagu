import React from 'react';
import {
  CheckCircle,
  Filter,
  ListChecks,
  Play,
  XCircle,
  StopCircle,
  Clock,
  Loader2,
} from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { RefreshButton } from '@/components/ui/refresh-button';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import DashboardTimeChart from '../features/dashboard/components/DashboardTimechart';
import { useQuery } from '../hooks/api';
// Import the main 'components' type and Status enum
import type { components } from '../api/v2/schema'; // Import the main components interface
import { Status } from '../api/v2/schema'; // Import the Status enum
import dayjs from '../lib/dayjs';

// Define types using the imported components structure
type DAGRunSummary = components['schemas']['DAGRunSummary'];

type Metrics = Record<Status, number>;

// Initialize metrics count for relevant statuses
const initializeMetrics = (): Metrics => {
  const initialMetrics: Partial<Metrics> = {};
  // Use only statuses defined in the enum
  const relevantStatuses = [
    Status.Success,
    Status.Failed,
    Status.Running,
    Status.Cancelled,
    Status.Queued,
    Status.NotStarted, // Include NotStarted if relevant
    Status.PartialSuccess,
  ];
  relevantStatuses.forEach((status: Status) => {
    initialMetrics[status] = 0;
  });
  return initialMetrics as Metrics;
};

// Ensure the function returns a React Element or null
function Dashboard(): React.ReactElement | null {
  // --- Hooks ---
  // All hooks must be called unconditionally at the top level.
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const [selectedDAGRun, setSelectedDAGRun] = React.useState<string>('all');
  const [dateRange, setDateRange] = React.useState<{
    startDate: number;
    endDate: number | undefined;
  }>(() => {
    // Initialize with today's date range
    const now = dayjs();
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');

    return {
      startDate: startOfDay.unix(),
      endDate: undefined, // No end date by default to get all dagRuns until now
    };
  });

  // Handle date change from the timeline component
  const handleDateChange = (startTimestamp: number, endTimestamp: number) => {
    setDateRange({
      startDate: startTimestamp,
      endDate: endTimestamp,
    });
  };

  const { data, error, isLoading, mutate } = useQuery('/dag-runs', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
        fromDate: dateRange.startDate,
        toDate: dateRange.endDate,
        name: selectedDAGRun !== 'all' ? selectedDAGRun : undefined,
      },
    },
    // Refresh every 5 seconds to keep the dashboard up-to-date
    refreshInterval: 5000,
  });

  // Extract unique dagRun names for the select dropdown - must be before conditional returns
  const dagRunsList: DAGRunSummary[] = data?.dagRuns || [];

  // This useMemo hook must be called unconditionally
  const uniqueDAGRunNames = React.useMemo(() => {
    const names = new Set<string>();
    if (data && data.dagRuns) {
      data.dagRuns.forEach((dagRun) => {
        if (dagRun.name) {
          names.add(dagRun.name);
        }
      });
    }
    return Array.from(names).sort();
  }, [data]);

  // Handle dagRun selection change
  const handleDAGRunChange = (value: string) => {
    setSelectedDAGRun(value);
  };

  // Effect for setting AppBar title - MUST be called before conditional returns
  React.useEffect(() => {
    // Ensure context is available before using it, although useContext should guarantee it here
    if (appBarContext) {
      appBarContext.setTitle('Dashboard');
    }
  }, [appBarContext]); // Dependency array includes the context

  // --- Conditional Returns ---
  // Handle error state
  if (error) {
    // Type assertion for the error object based on the default error schema
    const errorData = error as components['schemas']['Error'];
    const errorMessage =
      errorData?.message || 'Unknown error loading dashboard';
    return <div className="p-4 text-red-600">Error: {errorMessage}</div>;
  }

  // --- Calculate metrics ---
  // Initialize metrics
  const metrics = initializeMetrics();
  const totalDAGRuns = dagRunsList.length;

  // Calculate metrics from dagRun data
  dagRunsList.forEach((dagRun) => {
    if (
      dagRun &&
      Object.prototype.hasOwnProperty.call(metrics, dagRun.status)
    ) {
      const statusKey = dagRun.status as Status;
      metrics[statusKey]! += 1;
    }
  });

  // --- Define metric cards data ---
  const metricCards = [
    {
      title: 'Total',
      value: totalDAGRuns,
      icon: <ListChecks className="h-5 w-5 text-muted-foreground" />,
    },
    {
      title: 'Running',
      value: metrics[Status.Running],
      icon: <Play className="h-5 w-5 text-[limegreen]" />,
    },
    {
      title: 'Queued',
      value: metrics[Status.Queued],
      icon: <Clock className="h-5 w-5 text-[purple]" />,
    },
    {
      title: 'Success',
      value: metrics[Status.Success],
      icon: <CheckCircle className="h-5 w-5 text-[green]" />,
    },
    {
      title: 'Partial Success',
      value: metrics[Status.PartialSuccess],
      icon: <CheckCircle className="h-5 w-5 text-[#f59e0b]" />,
    },
    {
      title: 'Failed',
      value: metrics[Status.Failed],
      icon: <XCircle className="h-5 w-5 text-[red]" />,
    },
    {
      title: 'Cancelled',
      value: metrics[Status.Cancelled],
      icon: <StopCircle className="h-5 w-5 text-[deeppink]" />,
    },
  ];

  let title = 'Timeline';
  if (config.tz) {
    title = `Timeline in ${config.tz}`;
  }

  // --- Render the dashboard UI ---
  return (
    <div className="flex flex-col gap-3 w-full h-full overflow-hidden">
      {/* Dense Header with Filters and Metrics */}
      <div className="border rounded bg-card flex-shrink-0">
        {/* Top row: Filters */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-3 border-b">
          <div className="flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="flex items-center gap-2">
              <Filter className="h-4 w-4 text-muted-foreground" />
              <span className="text-xs font-medium text-muted-foreground">
                DAG Name:
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Select
                value={selectedDAGRun}
                onValueChange={handleDAGRunChange}
                disabled={isLoading}
              >
                <SelectTrigger className="h-7 w-full sm:w-[180px] text-xs">
                  <SelectValue
                    placeholder={isLoading ? 'Loading...' : 'All dagRuns'}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all" className="text-xs">
                    All
                  </SelectItem>
                  {uniqueDAGRunNames.map((name) => (
                    <SelectItem key={name} value={name} className="text-xs">
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedDAGRun !== 'all' && (
                <span className="text-xs text-muted-foreground px-2 py-1 bg-muted rounded whitespace-nowrap">
                  {selectedDAGRun}
                </span>
              )}
            </div>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                Date:
              </span>
              <div className="relative">
                <Input
                  type="date"
                  value={dayjs.unix(dateRange.startDate).format('YYYY-MM-DD')}
                  onChange={(e) => {
                    const newDate = e.target.value;
                    if (!newDate) return; // Handle empty input

                    const date = dayjs(newDate);
                    if (!date.isValid()) return; // Handle invalid dates

                    const startOfDay =
                      config.tzOffsetInSec !== undefined
                        ? date
                            .utcOffset(config.tzOffsetInSec / 60)
                            .startOf('day')
                        : date.startOf('day');
                    const endOfDay =
                      config.tzOffsetInSec !== undefined
                        ? date.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                        : date.endOf('day');
                    handleDateChange(startOfDay.unix(), endOfDay.unix());
                  }}
                  className="h-7 w-[140px] text-xs pr-8"
                />
                {isLoading && (
                  <Loader2 className="absolute right-2 top-1.5 h-4 w-4 animate-spin text-muted-foreground" />
                )}
              </div>
              <Button
                size="sm"
                onClick={() => {
                  const now = dayjs();
                  const startOfDay =
                    config.tzOffsetInSec !== undefined
                      ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
                      : now.startOf('day');
                  const endOfDay =
                    config.tzOffsetInSec !== undefined
                      ? now.utcOffset(config.tzOffsetInSec / 60).endOf('day')
                      : now.endOf('day');
                  handleDateChange(startOfDay.unix(), endOfDay.unix());
                }}
                className="px-4"
              >
                Today
              </Button>
              <RefreshButton 
                onRefresh={async () => { await mutate(); }} 
              />
            </div>
          </div>
        </div>

        {/* Bottom row: Dense metrics */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 divide-x divide-y lg:divide-y-0">
          {metricCards.map((card) => (
            <div
              key={card.title}
              className="p-2 sm:p-3 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 sm:gap-2"
            >
              <div className="flex items-center gap-1 sm:gap-2">
                {React.cloneElement(card.icon, {
                  className: card.icon.props.className.replace(
                    'h-5 w-5',
                    'h-3 w-3'
                  ),
                })}
                <span className="text-xs font-medium text-muted-foreground">
                  {card.title}
                </span>
              </div>
              <span className="text-lg font-bold">{card.value}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Compact Timeline Chart */}
      <div className="border rounded bg-card flex-1 flex flex-col min-h-0">
        <div className="flex items-center justify-between p-3 border-b flex-shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold">{title}</span>
          </div>
        </div>
        <div className="flex-1 min-h-0">
          <DashboardTimeChart
            data={dagRunsList}
            selectedDate={{
              startTimestamp: dateRange.startDate,
              endTimestamp: dateRange.endDate,
            }}
          />
        </div>
      </div>
    </div>
  );
}

export default Dashboard;
