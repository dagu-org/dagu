import type { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import { useQuery } from '@/hooks/api';
import {
  liveFallbackOptions,
  useLiveConnection,
  useLiveDAGRuns,
} from '@/hooks/useAppLive';
import dayjs from '@/lib/dayjs';
import { ArrowUpRight, ChevronLeft, ChevronRight, Clock3, Search } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';
import Title from '@/ui/Title';

type RecentEventEntry = components['schemas']['RecentEventEntry'];
type RecentEventType = components['schemas']['RecentEventType'];

const PAGE_SIZE = 50;

const EVENT_TYPES: Array<{ value: 'all' | RecentEventType; label: string }> = [
  { value: 'all', label: 'All Types' },
  { value: 'waiting', label: 'Waiting' },
  { value: 'approved', label: 'Approved' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'push_back', label: 'Push Back' },
  { value: 'failed', label: 'Failed' },
  { value: 'aborted', label: 'Aborted' },
];

function formatEventType(value: RecentEventType): string {
  switch (value) {
    case 'push_back':
      return 'Push Back';
    default:
      return value.charAt(0).toUpperCase() + value.slice(1);
  }
}

function typeBadgeClass(value: RecentEventType): string {
  switch (value) {
    case 'failed':
    case 'rejected':
    case 'aborted':
      return 'bg-destructive/10 text-destructive';
    case 'waiting':
      return 'bg-amber-500/10 text-amber-700 dark:text-amber-300';
    case 'approved':
      return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
    case 'push_back':
      return 'bg-sky-500/10 text-sky-700 dark:text-sky-300';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

export default function RecentEventsPage() {
  const config = useConfig();
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [search, setSearch] = React.useState('');
  const [dagName, setDagName] = React.useState('');
  const [actor, setActor] = React.useState('');
  const [type, setType] = React.useState<'all' | RecentEventType>('all');
  const [fromDate, setFromDate] = React.useState('');
  const [toDate, setToDate] = React.useState('');
  const [offset, setOffset] = React.useState(0);

  const deferredSearch = React.useDeferredValue(search);
  const deferredDagName = React.useDeferredValue(dagName);
  const deferredActor = React.useDeferredValue(actor);

  React.useEffect(() => {
    appBarContext.setTitle('Recent Events');
  }, [appBarContext]);

  React.useEffect(() => {
    setOffset(0);
  }, [deferredSearch, deferredDagName, deferredActor, type, fromDate, toDate, remoteNode]);

  const formatDateForApi = React.useCallback(
    (value: string): string | undefined => {
      if (!value) return undefined;
      if (config.tzOffsetInSec !== undefined) {
        return dayjs(value)
          .utcOffset(config.tzOffsetInSec / 60, true)
          .toISOString();
      }
      return dayjs(value).toISOString();
    },
    [config.tzOffsetInSec]
  );

  const liveState = useLiveConnection();
  const query = React.useMemo(
    () => ({
      remoteNode,
      limit: PAGE_SIZE,
      offset,
      search: deferredSearch || undefined,
      dagName: deferredDagName || undefined,
      actor: deferredActor || undefined,
      type: type === 'all' ? undefined : type,
      startTime: formatDateForApi(fromDate),
      endTime: formatDateForApi(toDate),
    }),
    [deferredActor, deferredDagName, deferredSearch, formatDateForApi, fromDate, offset, remoteNode, toDate, type]
  );

  const { data, error, isLoading, mutate } = useQuery(
    '/recent-events',
    { params: { query } },
    liveFallbackOptions(liveState, 3000)
  );
  useLiveDAGRuns(mutate);

  const entries = data?.entries || [];
  const total = data?.total || 0;
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function buildRunHref(entry: RecentEventEntry): string {
    if (!entry.subDAGRunId) {
      return `/dag-runs/${entry.dagName}/${entry.dagRunId}`;
    }
    const searchParams = new URLSearchParams({
      subDAGRunId: entry.subDAGRunId,
      dagRunId: entry.dagRunId,
      dagRunName: entry.dagName,
    });
    return `/dag-runs/${entry.dagName}/${entry.dagRunId}?${searchParams.toString()}`;
  }

  function formatTimestamp(timestamp: string): string {
    const parsed = dayjs(timestamp);
    if (config.tzOffsetInSec !== undefined) {
      return parsed.utcOffset(config.tzOffsetInSec / 60).format('MMM D, YYYY HH:mm:ss');
    }
    return parsed.format('MMM D, YYYY HH:mm:ss');
  }

  function formatDetails(entry: RecentEventEntry): string {
    const parts: string[] = [];
    if (entry.stepName) {
      parts.push(`Step: ${entry.stepName}`);
    }
    if (entry.reason) {
      parts.push(`Reason: ${entry.reason}`);
    }
    if (entry.resultingRunStatus) {
      parts.push(`Run: ${entry.resultingRunStatus}`);
    }
    if (entry.approvalIteration !== undefined) {
      parts.push(`Iteration: ${entry.approvalIteration}`);
    }
    if (entry.resumed !== undefined) {
      parts.push(entry.resumed ? 'Resumed' : 'Not resumed');
    }
    return parts.join(' • ') || '-';
  }

  async function handleRefresh(): Promise<void> {
    await mutate();
  }

  return (
    <div className="flex flex-col gap-4 max-w-7xl h-full overflow-hidden">
      <Title>Recent Events</Title>

      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 flex-shrink-0">
        <div>
          <h1 className="text-lg font-semibold">Recent Events</h1>
          <p className="text-sm text-muted-foreground">
            Review recent workflow lifecycle events and jump back to the affected run.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void handleRefresh()}>
          Refresh
        </Button>
      </div>

      <div className="flex flex-col gap-2 xl:flex-row xl:items-center xl:justify-between flex-shrink-0">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2 top-2 h-3.5 w-3.5 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search events..."
              className="w-[220px] pl-8 h-8"
            />
          </div>
          <Input
            value={dagName}
            onChange={(e) => setDagName(e.target.value)}
            placeholder="Filter by DAG"
            className="w-[180px] h-8"
          />
          <Input
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            placeholder="Filter by actor"
            className="w-[160px] h-8"
          />
          <Select value={type} onValueChange={(value) => setType(value as 'all' | RecentEventType)}>
            <SelectTrigger className="w-[160px] h-8">
              <SelectValue placeholder="All Types" />
            </SelectTrigger>
            <SelectContent>
              {EVENT_TYPES.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Input
            type="datetime-local"
            value={fromDate}
            onChange={(e) => setFromDate(e.target.value)}
            className="w-[220px] h-8"
          />
          <Input
            type="datetime-local"
            value={toDate}
            onChange={(e) => setToDate(e.target.value)}
            className="w-[220px] h-8"
          />
        </div>
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 text-destructive text-sm px-3 py-2">
          {(error as { message?: string })?.message || 'Failed to load recent events'}
        </div>
      )}

      <div className="border border-border rounded-md flex-1 min-h-0 flex flex-col overflow-hidden bg-background">
        <div className="flex-shrink-0 border-b border-border bg-background">
          <table className="w-full table-fixed">
            <thead>
              <tr>
                <th className="w-[180px] px-4 py-3 text-left text-sm font-medium text-muted-foreground">Timestamp</th>
                <th className="w-[120px] px-4 py-3 text-left text-sm font-medium text-muted-foreground">Type</th>
                <th className="w-[220px] px-4 py-3 text-left text-sm font-medium text-muted-foreground">Run</th>
                <th className="w-[140px] px-4 py-3 text-left text-sm font-medium text-muted-foreground">Actor</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">Details</th>
              </tr>
            </thead>
          </table>
        </div>
        <div className="flex-1 min-h-0 overflow-auto">
          <table className="w-full table-fixed">
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-muted-foreground">
                    Loading recent events...
                  </td>
                </tr>
              ) : entries.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-muted-foreground">
                    <Clock3 className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    No recent events found
                  </td>
                </tr>
              ) : (
                entries.map((entry) => (
                  <tr key={entry.id} className="border-b border-border hover:bg-muted/40">
                    <td className="w-[180px] px-4 py-3 text-sm text-muted-foreground whitespace-nowrap">
                      {formatTimestamp(entry.timestamp)}
                    </td>
                    <td className="w-[120px] px-4 py-3">
                      <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${typeBadgeClass(entry.type)}`}>
                        {formatEventType(entry.type)}
                      </span>
                    </td>
                    <td className="w-[220px] px-4 py-3 text-sm">
                      <Link
                        to={buildRunHref(entry)}
                        className="inline-flex items-center gap-1 text-primary hover:underline underline-offset-2"
                      >
                        <span className="truncate">
                          {entry.dagName}/{entry.subDAGRunId || entry.dagRunId}
                        </span>
                        <ArrowUpRight className="h-3.5 w-3.5 flex-shrink-0" />
                      </Link>
                    </td>
                    <td className="w-[140px] px-4 py-3 text-sm text-muted-foreground">
                      {entry.actor || '-'}
                    </td>
                    <td className="px-4 py-3 text-sm text-muted-foreground" title={formatDetails(entry)}>
                      <div className="truncate">{formatDetails(entry)}</div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between flex-shrink-0">
          <p className="text-sm text-muted-foreground">
            Showing {offset + 1} - {Math.min(offset + PAGE_SIZE, total)} of {total} events
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
            >
              <ChevronLeft className="h-4 w-4 mr-1" />
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {currentPage} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total}
            >
              Next
              <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
