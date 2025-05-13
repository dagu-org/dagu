import React from 'react';
// Assuming the path alias is correct and the component exists
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { CheckCircle, ListChecks, Play, XCircle } from 'lucide-react';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import DashboardTimeChart from '../features/dashboard/components/DashboardTimechart';
import { useQuery } from '../hooks/api';
import Title from '../ui/Title';
// Import the main 'components' type and Status enum
import type { components } from '../api/v2/schema'; // Import the main components interface
import { Status } from '../api/v2/schema'; // Import the Status enum
import dayjs from '../lib/dayjs';

// Define types using the imported components structure
type WorkflowSummary = components['schemas']['WorkflowSummary'];

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
    Status.NotStarted, // Include NotStarted if relevant
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
  // Calculate the start of today in the configured timezone
  const getStartOfTodayTimestamp = (): number => {
    const now = dayjs();
    // Apply timezone offset and set to beginning of day (00:00)
    const startOfDay =
      config.tzOffsetInSec !== undefined
        ? now.utcOffset(config.tzOffsetInSec / 60).startOf('day')
        : now.startOf('day');

    // Return as Unix timestamp (seconds)
    return startOfDay.unix();
  };

  const { data, error, isLoading } = useQuery('/workflows', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
        fromDate: getStartOfTodayTimestamp(),
      },
    },
    // Refresh every 5 seconds to keep the dashboard up-to-date
    refreshInterval: 5000,
  });

  // Effect for setting AppBar title - MUST be called before conditional returns
  React.useEffect(() => {
    // Ensure context is available before using it, although useContext should guarantee it here
    if (appBarContext) {
      appBarContext.setTitle('Dashboard');
    }
  }, [appBarContext]); // Dependency array includes the context

  // --- Conditional Returns ---
  // Handle loading state
  if (isLoading) {
    return <div className="p-4">Loading dashboard data...</div>; // Or use a spinner component
  }

  // Handle error state
  if (error) {
    // Type assertion for the error object based on the default error schema
    const errorData = error as components['schemas']['Error'];
    const errorMessage =
      errorData?.message || 'Unknown error loading dashboard';
    return <div className="p-4 text-red-600">Error: {errorMessage}</div>;
  }

  // Handle case where data might be null/undefined after loading
  if (!data) {
    return <div className="p-4">No dashboard data received.</div>;
  }

  // --- Calculate metrics ---
  // This logic runs only if data is available (after conditional returns)
  const metrics = initializeMetrics();
  const workflowsList: WorkflowSummary[] = data.workflows || []; // Access workflows from the successfully loaded data
  const totalWorkflows = workflowsList.length;

  workflowsList.forEach((workflow) => {
    if (
      workflow &&
      Object.prototype.hasOwnProperty.call(metrics, workflow.status)
    ) {
      const statusKey = workflow.status as Status;
      metrics[statusKey]! += 1;
    }
  });

  // --- Define metric cards data ---
  const metricCards = [
    {
      title: 'Total Workflows',
      value: totalWorkflows,
      icon: <ListChecks className="h-5 w-5 text-muted-foreground" />,
    },
    {
      title: 'Running',
      value: metrics[Status.Running],
      icon: <Play className="h-5 w-5 text-muted-foreground" />,
    },
    {
      title: 'Successful',
      value: metrics[Status.Success],
      icon: <CheckCircle className="h-5 w-5 text-muted-foreground" />,
    },
    {
      title: 'Failed',
      value: metrics[Status.Failed],
      icon: <XCircle className="h-5 w-5 text-muted-foreground" />,
    },
  ];

  let title = 'Timeline';
  if (config.tz) {
    title = `Timeline in ${config.tz}`;
  }

  // --- Render the dashboard UI ---
  return (
    <div className="flex flex-col space-y-6 w-full">
      {/* Metric Cards Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {metricCards.map((card) => (
          <Card key={card.title}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                {card.title}
              </CardTitle>
              {card.icon}
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{card.value}</div>
              {/* Optional: Add description or trend */}
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Timeline Chart Section */}
      <div className="rounded-lg border bg-card text-card-foreground shadow-sm p-4 md:p-6">
        <Title>{title}</Title>
        {/* Remove fixed height (h-[300px]) to allow vertical expansion */}
        {/* Add overflow-x-auto to allow horizontal scrolling if chart is too wide */}
        <div className="mt-4 overflow-x-auto">
          {' '}
          {/* Adjust height as needed */}
          <DashboardTimeChart data={workflowsList} />
        </div>
      </div>
    </div>
  );
}

export default Dashboard;
