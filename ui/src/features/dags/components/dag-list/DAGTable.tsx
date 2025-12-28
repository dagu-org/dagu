import {
  Column,
  ColumnFiltersState,
  createColumnHelper,
  ExpandedState,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  RowData,
  Updater,
  useReactTable,
} from '@tanstack/react-table';
import cronParser, { CronDate } from 'cron-parser';
import {
  ArrowDown,
  ArrowUp,
  Calendar,
  ChevronDown,
  ChevronUp,
  Filter,
  Search,
} from 'lucide-react';
import React, { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Ticker from '../../../../ui/Ticker';
import VisuallyHidden from '../../../../ui/VisuallyHidden';
import { CreateDAGModal, DAGPagination } from '../common';
import DAGActions from '../common/DAGActions';
import LiveSwitch from '../common/LiveSwitch';

// Helper to format milliseconds into d/h/m/s
function formatMs(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  // Only show seconds if no larger units or if seconds > 0
  if (secs > 0 || parts.length === 0) parts.push(`${secs}s`);
  return parts.join(' ');
}

// Import shadcn/ui components
import { Badge } from '../../../../components/ui/badge';
import { Button } from '../../../../components/ui/button';
import { Input } from '../../../../components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../../components/ui/select'; // Use shadcn Select
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../../../components/ui/table';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '../../../../components/ui/tooltip';
import { PanelWidthContext } from '../../../../components/SplitLayout';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';

// Threshold in pixels below which we switch to card view
// Set higher than table's comfortable minimum width (~700px for all columns)
const CARD_VIEW_THRESHOLD = 700;

// Reusable DAG Card component for mobile and narrow panel views
interface DAGCardProps {
  dag: components['schemas']['DAGFile'];
  isSelected: boolean;
  onSelect: (fileName: string, title: string) => void;
  onTagClick: (tag: string) => void;
  refreshFn: () => void;
  className?: string;
}

function DAGCard({ dag, isSelected, onSelect, onTagClick, refreshFn, className = '' }: DAGCardProps) {
  const fileName = dag.fileName;
  const title = dag.dag.name;
  const status = dag.latestDAGRun.status;
  const statusLabel = dag.latestDAGRun.statusLabel;
  const tags = dag.dag.tags || [];
  const description = dag.dag.description;
  const schedules = dag.dag.schedule || [];
  const hasSchedule = schedules.length > 0;

  const handleCardClick = (e: React.MouseEvent) => {
    if (e.metaKey || e.ctrlKey) {
      window.open(`/dags/${fileName}`, '_blank');
    } else {
      onSelect(fileName, title);
    }
  };

  return (
    <div
      className={`p-2.5 rounded-md border cursor-pointer overflow-hidden ${
        isSelected
          ? 'bg-primary/10 border-primary'
          : 'bg-card border-border hover:bg-muted/50'
      } ${className}`}
      onClick={handleCardClick}
    >
      {/* Header: Name + Status */}
      <div className="flex justify-between items-start gap-2 mb-1.5">
        <div className="font-medium text-xs truncate flex-1 min-w-0">
          {dag.dag.name}
        </div>
        <StatusChip status={status} size="xs">
          {statusLabel}
        </StatusChip>
      </div>

      {/* Description */}
      {description && (
        <div className="text-[10px] text-muted-foreground mb-1.5 line-clamp-1">
          {description}
        </div>
      )}

      {/* Schedule & Last Run */}
      <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground mb-1.5">
        {schedules.map((schedule, idx) => (
          <Badge
            key={idx}
            variant="outline"
            className="text-[9px] font-normal px-1 py-0 h-3"
          >
            {schedule.expression}
          </Badge>
        ))}
        {dag.latestDAGRun.startedAt && dag.latestDAGRun.startedAt !== '-' && (
          <span className="flex items-center gap-0.5">
            <Calendar className="h-2.5 w-2.5" />
            <span className="text-[9px]">{dag.latestDAGRun.startedAt}</span>
          </span>
        )}
      </div>

      {/* Tags */}
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-0.5 mb-1.5">
          {tags.map((tag) => (
            <Badge
              key={tag}
              variant="outline"
              className="text-[9px] px-1 py-0 h-3 rounded-sm border-primary/30 bg-primary/10 text-primary"
              onClick={(e) => {
                e.stopPropagation();
                onTagClick(tag);
              }}
            >
              <div className="h-1 w-1 rounded-full bg-primary/70 mr-0.5"></div>
              {tag}
            </Badge>
          ))}
        </div>
      )}

      {/* Actions row: LiveSwitch + DAGActions */}
      <div className="flex flex-wrap items-center justify-between gap-1 pt-1.5 border-t border-border/50 min-w-0">
        <div
          className={`flex items-center gap-1 flex-shrink-0 min-w-0 ${!hasSchedule ? 'opacity-40 pointer-events-none' : ''}`}
          onClick={(e) => e.stopPropagation()}
        >
          <LiveSwitch dag={dag} refresh={refreshFn} />
          <span className="text-[9px] text-muted-foreground truncate">
            {dag.suspended ? 'Suspended' : hasSchedule ? 'Live' : 'No schedule'}
          </span>
        </div>
        <div className="flex-shrink-0" onClick={(e) => e.stopPropagation()}>
          <DAGActions
            dag={dag.dag}
            status={dag.latestDAGRun}
            fileName={fileName}
            label={false}
            refresh={refreshFn}
          />
        </div>
      </div>
    </div>
  );
}

/**
 * Props for the DAGTable component
 */
type Props = {
  /** List of DAG files to display */
  dags: components['schemas']['DAGFile'][];
  /** Current group filter */
  group: string;
  /** Function to refresh the data */
  refreshFn: () => void;
  /** Current search text */
  searchText: string;
  /** Handler for search text changes */
  handleSearchTextChange: (searchText: string) => void;
  /** Current tag filter */
  searchTag: string;
  /** Handler for tag filter changes */
  handleSearchTagChange: (tag: string) => void;
  /** Loading state */
  isLoading?: boolean;
  /** Pagination props */
  pagination?: {
    /** Total number of pages */
    totalPages: number;
    /** Current page number */
    page: number;
    /** Number of items per page */
    pageLimit: number;
    /** Callback for page change */
    pageChange: (page: number) => void;
    /** Callback for page limit change */
    onPageLimitChange: (pageLimit: number) => void;
  };
  /** Current sort field */
  sortField?: string;
  /** Current sort order */
  sortOrder?: string;
  /** Handler for sort changes */
  onSortChange?: (field: string, order: string) => void;
  /** Currently selected DAG file name */
  selectedDAG?: string | null;
  /** Handler for DAG selection changes */
  onSelectDAG?: (fileName: string, title: string) => void;
};

/**
 * Types for table rows
 */
type RowItem = DAGRow | GroupRow;
type DAGRow = {
  kind: ItemKind;
  name: string;
  dag: components['schemas']['DAGFile'];
};
type GroupRow = {
  kind: ItemKind.Group;
  name: string;
};
enum ItemKind {
  DAG = 0,
  Group,
}
type Data = RowItem & { subRows?: RowItem[] };

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface TableMeta<TData extends RowData> {
    group: string;
    refreshFn: () => void;
    // Add tag change handler to meta for direct access in cell
    handleSearchTagChange?: (tag: string) => void;
  }
}

const columnHelper = createColumnHelper<Data>();

function getTzAndExp(exp: string) {
  const parts = exp.trim().split(/\s+/);

  if (parts[0]?.startsWith('CRON_TZ=')) {
    const timezone = parts[0]?.split('=')[1];
    const cronExpr = parts?.slice(1).join(' ');
    return [timezone, cronExpr];
  } else {
    return [parts.join(' ')];
  }
}

function getNextSchedule(
  data: components['schemas']['DAGFile']
): CronDate | undefined {
  const schedules = data.dag.schedule;
  if (!schedules || schedules.length === 0 || data.suspended) {
    return;
  }
  try {
    const datesToRun = schedules.map((schedule) => {
      const parsedCronExp = getTzAndExp(schedule.expression);
      const options = {
        tz: parsedCronExp.length > 1 ? parsedCronExp[0] : getConfig().tz,
        iterator: true,
      };
      // Assuming 'parseExpression' is the correct method name based on library docs
      const cronExp =
        parsedCronExp.length > 1 ? parsedCronExp[1] : parsedCronExp[0];
      const interval = cronParser.parse(cronExp!, options);
      return interval.next();
    });
    // Sort the next run dates
    datesToRun.sort((a, b) => a.getTime() - b.getTime());
    // Return the earliest next run date
    if (datesToRun[0]) {
      return datesToRun[0];
    }
    return;
  } catch (e) {
    console.error('Error parsing cron expression:', e);
    return;
  }
}
// --- End Helper Functions ---

const defaultColumns = [
  columnHelper.accessor('name', {
    id: 'Expand',
    header: ({ table }) => (
      <div
        role="button"
        tabIndex={0}
        onClick={table.getToggleAllRowsExpandedHandler()}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            table.toggleAllRowsExpanded();
          }
        }}
        className="flex items-center justify-center text-muted-foreground cursor-pointer h-6 w-6"
      >
        {table.getIsAllRowsExpanded() ? (
          <>
            <VisuallyHidden>Compress rows</VisuallyHidden>
            <ChevronUp className="h-4 w-4" />
          </>
        ) : (
          <>
            <VisuallyHidden>Expand rows</VisuallyHidden>
            <ChevronDown className="h-4 w-4" />
          </>
        )}
      </div>
    ),
    cell: ({ row }) => {
      if (row.getCanExpand()) {
        return (
          <div
            className="flex items-center justify-center min-h-[2.5rem] text-muted-foreground cursor-pointer"
            role="button"
            tabIndex={0}
            onClick={(e) => {
              e.stopPropagation();
              row.toggleExpanded();
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.stopPropagation();
                e.preventDefault();
                row.toggleExpanded();
              }
            }}
          >
            {row.getIsExpanded() ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </div>
        );
      }
      return null;
    },
    size: 32,
    minSize: 32,
    maxSize: 32,
  }),
  columnHelper.accessor('name', {
    id: 'Name',
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Name</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Description
        </span>
      </div>
    ),
    cell: ({ row, getValue, table }) => {
      const data = row.original!;

      if (data.kind === ItemKind.Group) {
        // Group Row: Render group name directly with vertical centering
        return (
          <div style={{ paddingLeft: `${row.depth * 1.5}rem` }} className="flex items-center min-h-[2.5rem]">
            <span className="font-normal text-muted-foreground">
              {getValue()}
            </span>
          </div>
        );
      } else {
        // DAG Row: Render link with description and tags below
        const tags = data.dag.dag.tags || [];
        const description = data.dag.dag.description;

        return (
          <div
            style={{ paddingLeft: `${row.depth * 1.5}rem` }}
            className="space-y-0.5 min-w-0"
          >
            <div className="font-medium text-foreground tracking-tight text-xs truncate">
              {getValue()}
            </div>

            {description && (
              <div className="text-[10px] text-muted-foreground whitespace-normal leading-tight line-clamp-2">
                {description}
              </div>
            )}

            {tags.length > 0 && (
              <div className="flex flex-wrap gap-0.5">
                {tags.map((tag) => (
                  <Badge
                    key={tag}
                    variant="outline"
                    className="text-[10px] px-1 py-0 h-3.5 rounded-sm border-primary/30 bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary transition-colors duration-200 cursor-pointer font-normal"
                    onClick={(e) => {
                      e.stopPropagation(); // Prevent row click
                      e.preventDefault();
                      // Get the handleSearchTagChange from the component props
                      const handleTagClick =
                        table.options.meta?.handleSearchTagChange;
                      if (handleTagClick) handleTagClick(tag);
                    }}
                  >
                    <div className="h-1 w-1 rounded-full bg-primary/70 mr-0.5"></div>
                    {tag}
                  </Badge>
                ))}
              </div>
            )}
          </div>
        );
      }
    },
    filterFn: (row, _, filterValue) => {
      // Use row instead of props
      const data = row.original!;
      if (data.kind === ItemKind.Group) {
        return true; // Always show group rows during filtering
      }
      if (data.kind === ItemKind.DAG) {
        const name = data.dag.dag.name.toLowerCase();
        const fileName = data.dag.fileName.toLowerCase();
        const description = (data.dag.dag.description || '').toLowerCase();
        const searchValue = String(filterValue).toLowerCase();

        // Search in name and description
        if (
          fileName.includes(searchValue) ||
          name.includes(searchValue) ||
          description.includes(searchValue)
        ) {
          return true;
        }

        // Also search in tags if needed
        const tags = data.dag.dag.tags || [];
        if (tags.some((tag) => tag.toLowerCase().includes(searchValue))) {
          return true;
        }
      }
      return false;
    },
  }),
  // Tags column removed as tags are now displayed under the name
  // The filter functionality is preserved in the Name column
  columnHelper.accessor('kind', {
    id: 'Status',
    size: 80,
    minSize: 80,
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Status</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Latest status
        </span>
      </div>
    ),
    cell: ({ row }) => {
      // Use row
      const data = row.original!;
      if (data.kind === ItemKind.DAG) {
        // Use the updated StatusChip component with xs size
        return (
          <StatusChip status={data.dag.latestDAGRun.status} size="xs">
            {data.dag.latestDAGRun?.statusLabel}
          </StatusChip>
        );
      }
      return null;
    },
  }),
  // Removed Started At and Finished At columns
  columnHelper.accessor('kind', {
    id: 'LastRun',
    size: 110,
    minSize: 90,
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Last Run</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          {getConfig().tz || 'Local Timezone'}
        </span>
      </div>
    ),
    cell: ({ row }) => {
      const data = row.original!;
      if (data.kind !== ItemKind.DAG) {
        return null;
      }

      const { startedAt, finishedAt, status } = data.dag.latestDAGRun;

      if (!startedAt || startedAt === '-') {
        // If no start time, display nothing or a placeholder
        return <span className="font-normal text-muted-foreground">-</span>;
      }

      const formattedStartedAt = startedAt;
      let durationContent: React.ReactNode = null;

      if (finishedAt && finishedAt !== '-') {
        const start = dayjs(startedAt);
        const end = dayjs(finishedAt);

        if (start.isValid() && end.isValid()) {
          const durationMs = end.diff(start);

          if (durationMs > 0) {
            durationContent = (
              <div className="text-[10px] text-muted-foreground">
                {formatMs(durationMs)}
              </div>
            );
          }
        }
      } else if (status === 1) {
        // Status 1 typically means "Running"
        durationContent = (
          <div className="text-[10px] text-muted-foreground">(Running)</div>
        );
      }

      return (
        <div className="space-y-0.5 min-w-0">
          <div className="font-normal text-foreground/70 text-xs truncate">
            {formattedStartedAt}
          </div>
          {durationContent}
        </div>
      );
    },
  }),
  columnHelper.accessor('kind', {
    id: 'ScheduleAndNextRun',
    size: 140,
    minSize: 120,
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Live / Schedule</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Toggle & next run
        </span>
      </div>
    ),
    cell: ({ row, table }) => {
      const data = row.original!;
      if (data.kind !== ItemKind.DAG) {
        return null;
      }

      const schedules = data.dag.dag.schedule || [];
      const hasSchedule = schedules.length > 0;

      // LiveSwitch component
      const liveSwitch = (
        <div
          onClick={(e) => e.stopPropagation()}
          className={`flex-shrink-0 p-0.5 ${!hasSchedule ? 'opacity-40 pointer-events-none' : ''}`}
        >
          <LiveSwitch
            dag={data.dag}
            refresh={table.options.meta?.refreshFn}
            aria-label={`Toggle ${data.name}`}
          />
        </div>
      );

      if (!hasSchedule) {
        return (
          <div className="flex items-center gap-2">
            {liveSwitch}
            <span className="text-[10px] text-muted-foreground">
              No schedule
            </span>
          </div>
        );
      }

      // Display schedule expressions
      const scheduleContent = (
        <div className="flex flex-wrap gap-0.5">
          {schedules.map((schedule) => (
            <Badge
              key={schedule.expression}
              variant="outline"
              className="text-[10px] font-normal px-1 py-0 h-3.5"
            >
              {schedule.expression}
            </Badge>
          ))}
        </div>
      );

      // Display next run information
      let nextRunContent: React.ReactNode | null = null;
      if (!data.dag.suspended && schedules.length > 0) {
        const nextRun = getNextSchedule(data.dag);
        if (nextRun) {
          nextRunContent = (
            <div className="text-[10px] text-muted-foreground font-normal leading-tight">
              <Ticker intervalMs={1000}>
                {() => {
                  const ms = nextRun.getTime() - new Date().getTime();
                  return <span>Run in {formatMs(ms)}</span>;
                }}
              </Ticker>
            </div>
          );
        }
      } else if (data.dag.suspended) {
        nextRunContent = (
          <div className="text-[10px] text-muted-foreground font-normal leading-tight">
            Suspended
          </div>
        );
      }

      return (
        <div className="flex items-start gap-1 min-w-0">
          {liveSwitch}
          <div className="space-y-0.5 min-w-0 overflow-hidden">
            {scheduleContent}
            {nextRunContent}
          </div>
        </div>
      );
    },
  }),
  columnHelper.display({
    id: 'Actions',
    size: 60,
    minSize: 60,
    maxSize: 60,
    header: () => (
      <div className="flex flex-col items-center py-1">
        <span className="text-xs">Actions</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Operations
        </span>
      </div>
    ),
    cell: ({ row, table }) => {
      // Use row and table
      const data = row.original!;
      if (data.kind === ItemKind.Group) {
        return null;
      }
      // Assuming DAGActions is refactored or compatible
      return (
        // Wrap DAGActions in a div and stop propagation on its click
        <div
          className="flex justify-center scale-90" // Scale down for density
          onClick={(e) => e.stopPropagation()}
        >
          <DAGActions
            dag={data.dag.dag}
            status={data.dag.latestDAGRun}
            fileName={data.dag.fileName}
            label={false}
            refresh={table.options.meta?.refreshFn}
          />
        </div>
      );
    },
  }),
];

// Mapping between column IDs and backend sort fields
const columnToSortField: Record<string, string> = {
  Name: 'name',
  ScheduleAndNextRun: 'nextRun',
};

// Client-side sortable columns
const clientSortableColumns = ['Status', 'LastRun'];

// --- Header Component for both Server-side and Client-side Sorting ---
const SortableHeader = ({
  column,
  children,
  currentSort,
  currentOrder,
  onSort,
  clientSort,
  clientOrder,
  onClientSort,
}: {
  column: Column<Data, unknown>;
  children: React.ReactNode;
  currentSort?: string;
  currentOrder?: string;
  onSort?: (field: string, order: string) => void;
  clientSort?: string;
  clientOrder?: string;
  onClientSort?: (field: string, order: string) => void;
}) => {
  const serverSortField = columnToSortField[column.id];
  const isClientSortable = clientSortableColumns.includes(column.id);

  // Check if this column is currently sorted (either server or client)
  const isServerActive = serverSortField && currentSort === serverSortField;
  const isClientActive = isClientSortable && clientSort === column.id;
  const isActive = isServerActive || isClientActive;

  // Determine if column is sortable at all
  const isSortable =
    (serverSortField && onSort) || (isClientSortable && onClientSort);

  if (!isSortable) {
    return <>{children}</>;
  }

  const handleClick = () => {
    if (serverSortField && onSort) {
      // Server-side sorting
      const newOrder =
        isServerActive && currentOrder === 'asc' ? 'desc' : 'asc';
      onSort(serverSortField, newOrder);
      // Clear client sort when server sort is applied
      if (onClientSort) {
        onClientSort('', '');
      }
    } else if (isClientSortable && onClientSort) {
      // Client-side sorting
      const newOrder = isClientActive && clientOrder === 'asc' ? 'desc' : 'asc';
      onClientSort(column.id, newOrder);
    }
  };

  // Determine which order to show
  const displayOrder = isServerActive
    ? currentOrder
    : isClientActive
      ? clientOrder
      : '';

  const button = (
    <Button
      variant="ghost"
      onClick={handleClick}
      className="-ml-4 h-8 cursor-pointer" // Adjust spacing
    >
      {children}
      {isActive && displayOrder === 'asc' && (
        <ArrowUp className="ml-2 h-4 w-4" />
      )}
      {isActive && displayOrder === 'desc' && (
        <ArrowDown className="ml-2 h-4 w-4" />
      )}
    </Button>
  );

  // Wrap client-sortable columns with tooltip
  if (isClientSortable) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{button}</TooltipTrigger>
        <TooltipContent className="bg-muted text-muted-foreground border">
          <p className="text-xs">Sorts current page only</p>
        </TooltipContent>
      </Tooltip>
    );
  }

  return button;
};

/**
 * DAGTable component displays a table of DAGs with filtering, sorting, and grouping capabilities
 */
function DAGTable({
  dags = [],
  group = '', // Keep group prop if needed for external filtering/logic
  refreshFn,
  searchText,
  handleSearchTextChange,
  searchTag,
  handleSearchTagChange,
  isLoading = false,
  pagination,
  sortField = 'name',
  sortOrder = 'asc',
  onSortChange,
  selectedDAG = null,
  onSelectDAG,
}: Props) {
  const navigate = useNavigate();
  const [columns] = React.useState(() => [...defaultColumns]);
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    []
  );
  const [expanded, setExpanded] = React.useState<ExpandedState>(() => {
    try {
      const saved = localStorage.getItem('dagu_dag_table_expanded');
      return saved ? JSON.parse(saved) : {};
    } catch {
      return {};
    }
  });

  const handleExpandedChange = React.useCallback(
    (updater: Updater<ExpandedState>) => {
      setExpanded((prev) => {
        const next = typeof updater === 'function' ? updater(prev) : updater;
        localStorage.setItem('dagu_dag_table_expanded', JSON.stringify(next));
        return next;
      });
    },
    []
  );

  // State for client-side sorting
  const [clientSort, setClientSort] = React.useState<string>('');
  const [clientOrder, setClientOrder] = React.useState<string>('asc');

  // Handler for client-side sorting
  const handleClientSort = (field: string, order: string) => {
    setClientSort(field);
    setClientOrder(order);
  };

  // Handler for DAG selection
  const handleSelectDAG = (fileName: string, title: string) => {
    // Check if screen is small (less than 768px width)
    const isSmallScreen = window.innerWidth < 768;

    if (isSmallScreen) {
      // For small screens, navigate directly to the DAG details page
      navigate(`/dags/${fileName}`);
    } else if (onSelectDAG) {
      // For larger screens, call the selection handler
      onSelectDAG(fileName, title);
    }
  };

  // Update column filters based on external search props
  // Tags filtering is combined with Name filter since Name column's filterFn searches in tags too
  React.useEffect(() => {
    const nameFilter = columnFilters.find((f) => f.id === 'Name');

    // Combine searchText and searchTag for the Name filter
    const combinedFilter = searchTag || searchText || '';
    const currentValue = nameFilter?.value || '';

    let updated = false;
    const newFilters = [...columnFilters];

    if (combinedFilter !== currentValue) {
      const idx = newFilters.findIndex((f) => f.id === 'Name');
      if (combinedFilter) {
        if (idx > -1) newFilters[idx] = { id: 'Name', value: combinedFilter };
        else newFilters.push({ id: 'Name', value: combinedFilter });
      } else if (idx > -1) {
        newFilters.splice(idx, 1);
      }
      updated = true;
    }

    if (updated) {
      setColumnFilters(newFilters);
    }
  }, [searchText, searchTag, columnFilters]);

  // Transform the flat list of DAGs into a hierarchical structure with groups
  const data = useMemo(() => {
    // Apply client-side sorting if needed
    const sortedDags = [...dags];
    if (clientSort) {
      sortedDags.sort((a, b) => {
        let aValue: string | components['schemas']['Status'] = '';
        let bValue: string | components['schemas']['Status'] = '';

        if (clientSort === 'Status') {
          aValue = a.latestDAGRun?.status || '';
          bValue = b.latestDAGRun?.status || '';
        } else if (clientSort === 'LastRun') {
          aValue = a.latestDAGRun?.startedAt || '';
          bValue = b.latestDAGRun?.startedAt || '';
        }

        // Handle ascending/descending
        if (clientOrder === 'desc') {
          [aValue, bValue] = [bValue, aValue];
        }

        // Compare values
        if (aValue < bValue) return -1;
        if (aValue > bValue) return 1;
        return 0;
      });
    }

    const groups: { [key: string]: Data } = {};
    sortedDags.forEach((dag) => {
      const groupName = dag.dag.group; // Use groupName consistently
      if (groupName) {
        if (!groups[groupName]) {
          groups[groupName] = {
            kind: ItemKind.Group,
            name: groupName,
            subRows: [],
          };
        }
        groups[groupName].subRows!.push({
          kind: ItemKind.DAG,
          name: dag.dag.name,
          dag: dag,
        });
      }
    });

    // Sort sub-rows within groups if client sorting is active
    if (clientSort) {
      Object.values(groups).forEach((group) => {
        if (group.subRows) {
          group.subRows.sort((a, b) => {
            const aDag = (a as DAGRow).dag;
            const bDag = (b as DAGRow).dag;
            let aValue: string | components['schemas']['Status'] = '';
            let bValue: string | components['schemas']['Status'] = '';

            if (clientSort === 'Status') {
              aValue = aDag.latestDAGRun?.status || '';
              bValue = bDag.latestDAGRun?.status || '';
            } else if (clientSort === 'LastRun') {
              aValue = aDag.latestDAGRun?.startedAt || '';
              bValue = bDag.latestDAGRun?.startedAt || '';
            }

            if (clientOrder === 'desc') {
              [aValue, bValue] = [bValue, aValue];
            }

            if (aValue < bValue) return -1;
            if (aValue > bValue) return 1;
            return 0;
          });
        }
      });
    }

    const hierarchicalData: Data[] = Object.values(groups); // Get group objects
    // Add DAGs without a group
    sortedDags
      .filter((dag) => !dag.dag.group)
      .forEach((dag) => {
        hierarchicalData.push({
          kind: ItemKind.DAG,
          name: dag.dag.name,
          dag: dag,
        });
      });
    return hierarchicalData;
  }, [dags, clientSort, clientOrder]); // Added client sort dependencies

  // Create a ref to store the table instance for external access
  const tableInstanceRef = React.useRef<ReturnType<
    typeof useReactTable
  > | null>(null);

  // Expose navigation function for external keyboard handling
  React.useEffect(() => {
    if (!selectedDAG || !tableInstanceRef.current || !onSelectDAG) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      // Get all DAG rows from the sorted table rows (not groups)
      const sortedRows = tableInstanceRef.current?.getRowModel().rows || [];
      const dagRows = sortedRows
        .filter((row) => (row.original as Data)?.kind === ItemKind.DAG)
        .map((row) => ({
          fileName: (row.original as DAGRow).dag.fileName,
          title: (row.original as DAGRow).dag.dag.name,
        }));

      // Find current index
      const currentIndex = dagRows.findIndex(
        (item) => item.fileName === selectedDAG
      );
      if (currentIndex === -1) return;

      // Navigate with arrow keys
      if (event.key === 'ArrowDown' && currentIndex < dagRows.length - 1) {
        event.preventDefault();
        const nextDAG = dagRows[currentIndex + 1];
        if (nextDAG) {
          onSelectDAG(nextDAG.fileName, nextDAG.title);
        }
      } else if (event.key === 'ArrowUp' && currentIndex > 0) {
        event.preventDefault();
        const prevDAG = dagRows[currentIndex - 1];
        if (prevDAG) {
          onSelectDAG(prevDAG.fileName, prevDAG.title);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [selectedDAG, onSelectDAG]);

  const instance = useReactTable<Data>({
    data,
    columns,
    // Use stable IDs for persistence
    getRowId: (row) =>
      row.kind === ItemKind.Group
        ? `group:${row.name}`
        : `dag:${(row as DAGRow).dag.fileName}`,
    getSubRows: (row) => row.subRows,
    getCoreRowModel: getCoreRowModel<Data>(),
    // Disable client-side sorting as we're using server-side sorting
    manualSorting: true,
    getFilteredRowModel: getFilteredRowModel<Data>(),
    onColumnFiltersChange: setColumnFilters, // Let table manage internal filter state
    getExpandedRowModel: getExpandedRowModel<Data>(),
    autoResetExpanded: false, // Keep expanded state on data change
    state: {
      expanded,
      columnFilters, // Pass filters to table state
    },
    onExpandedChange: handleExpandedChange,
    // Pass handlers via meta
    meta: {
      group, // Pass group if needed elsewhere
      refreshFn,
      handleSearchTagChange, // Pass tag handler
    },
  });

  // Store the table instance in the ref with type assertion
  tableInstanceRef.current = instance as ReturnType<typeof useReactTable>;

  const appBarContext = React.useContext(AppBarContext);
  const panelWidth = React.useContext(PanelWidthContext);

  // Use card view when panel is narrow (below threshold) on desktop
  const useCardView = panelWidth !== null && panelWidth < CARD_VIEW_THRESHOLD;

  const { data: uniqueTags } = useQuery('/dags/tags', {
    params: {
      query: {
        remoteNode: appBarContext?.selectedRemoteNode || 'local',
      },
    },
  });

  return (
    <div className="space-y-2">
      {/* Search, Filter and Pagination Controls */}
      <div
        className={`bg-muted/50 rounded-lg mb-2 space-y-2 ${
          isLoading ? 'opacity-70 pointer-events-none' : ''
        }`}
      >
        <div className="flex flex-wrap items-center gap-2">
          {/* Search input */}
          <div className="relative flex-1 min-w-[120px] max-w-[280px]">
            <div className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
              <Search className="h-4 w-4" />
            </div>
            <Input
              type="text"
              placeholder="Search..."
              value={searchText}
              onChange={(e) => handleSearchTextChange(e.target.value)}
              className="pl-10 h-9 border border-border rounded-md w-full"
            />
            {searchText && (
              <button
                onClick={() => handleSearchTextChange('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                aria-label="Clear search"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <line x1="18" y1="6" x2="6" y2="18"></line>
                  <line x1="6" y1="6" x2="18" y2="18"></line>
                </svg>
              </button>
            )}
          </div>

          {/* Tag filter */}
          <Select
            value={searchTag}
            onValueChange={(value) =>
              handleSearchTagChange(value === 'all' ? '' : value)
            }
          >
            <SelectTrigger className="w-auto min-w-[80px] h-9 border border-border rounded-md flex-shrink-0">
              <div className="flex items-center gap-1">
                <Filter className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                <SelectValue placeholder="Tag" />
              </div>
            </SelectTrigger>
            <SelectContent className="max-h-[280px] overflow-y-auto">
              <SelectItem value="all">
                <span className="font-medium">All Tags</span>
              </SelectItem>
              {uniqueTags?.tags?.map((tag) => (
                <SelectItem key={tag} value={tag}>
                  {tag}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Pagination - pushed to right */}
          {pagination && (
            <div className="flex-shrink-0 ml-auto">
              <DAGPagination
                totalPages={pagination.totalPages}
                page={pagination.page}
                pageChange={pagination.pageChange}
                onPageLimitChange={pagination.onPageLimitChange}
                pageLimit={pagination.pageLimit}
              />
            </div>
          )}
        </div>
      </div>

      {/* Desktop Table View - Hidden on mobile or when panel is narrow */}
      <div className={`w-full overflow-hidden ${useCardView ? 'hidden' : 'hidden md:block'}`}>
        <Table
          className={`w-full text-xs ${isLoading ? 'opacity-70' : ''}`}
          style={{ tableLayout: 'fixed' }}
        >
{/* Column widths: Expand 32px fixed, Name auto, Status 10%, LastRun 18%, Schedule 20%, Actions 10% */}
          <colgroup><col style={{ width: '32px' }} /><col /><col style={{ width: '10%' }} /><col style={{ width: '18%' }} /><col style={{ width: '20%' }} /><col style={{ width: '10%' }} /></colgroup>
          <TableHeader>
            {instance.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header, index) => (
                  <TableHead
                    key={header.id}
                    className={`py-1 text-muted-foreground text-xs overflow-hidden ${index === 0 ? 'px-0' : 'px-2'}`}
                  >
                    {header.isPlaceholder ? null : (
                      <div>
                        {' '}
                        {/* Wrap header content */}
                        {columnToSortField[header.column.id] ||
                        clientSortableColumns.includes(header.column.id) ? (
                          <SortableHeader
                            column={header.column}
                            currentSort={sortField}
                            currentOrder={sortOrder}
                            onSort={onSortChange}
                            clientSort={clientSort}
                            clientOrder={clientOrder}
                            onClientSort={handleClientSort}
                          >
                            {flexRender(
                              header.column.columnDef.header,
                              header.getContext()
                            )}
                          </SortableHeader>
                        ) : (
                          flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )
                        )}
                      </div>
                    )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {instance.getRowModel().rows.length ? (
              instance.getRowModel().rows.map((row) => {
                // For DAG rows, make the entire row clickable
                const isDAGRow = row.original?.kind === ItemKind.DAG;
                // Type guard to ensure we only access dag property when it exists

                return (
                  <TableRow
                    key={row.id}
                    data-state={row.getIsSelected() && 'selected'}
                    className={`text-[0.8125rem] ${
                      row.original?.kind === ItemKind.Group
                        ? 'bg-muted/50 font-semibold cursor-pointer hover:bg-muted/70 border-l-4 border-transparent' // Make group rows clickable
                        : isDAGRow &&
                            'dag' in row.original &&
                            selectedDAG ===
                              (row.original as DAGRow).dag.fileName
                          ? 'cursor-pointer bg-primary/10 hover:bg-primary/15 border-l-4 border-primary' // Highlight selected DAG
                          : 'cursor-pointer hover:bg-muted/50 border-l-4 border-transparent'
                    }`}
                    onClick={(e) => {
                      // Handle group row clicks - toggle expanded state
                      if ((row.original as Data)?.kind === ItemKind.Group) {
                        row.toggleExpanded();
                      }
                      // Handle DAG row clicks - select DAG or open in new tab
                      else if (isDAGRow && 'dag' in row.original) {
                        const dagRow = row.original as DAGRow;
                        const fileName = dagRow.dag.fileName;
                        const title = dagRow.dag.dag.name;

                        // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, open in new tab
                        if (e.metaKey || e.ctrlKey) {
                          window.open(`/dags/${fileName}`, '_blank');
                        } else {
                          // Normal click behavior - select the DAG
                          handleSelectDAG(fileName, title);
                        }
                      }
                    }}
                  >
                    {row.getVisibleCells().map((cell, index) => (
                      <TableCell
                        key={cell.id}
                        className={`py-1 overflow-hidden align-middle truncate ${index === 0 ? 'px-0' : 'px-2'}`}
                      >
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext()
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                );
              })
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-64 text-center"
                >
                  <div className="flex flex-col items-center justify-center py-8">
                    <div className="text-6xl mb-4">🔍</div>
                    <h3 className="text-lg font-medium text-foreground mb-2">
                      No DAGs found
                    </h3>
                    <p className="text-sm text-muted-foreground text-center max-w-md mb-4">
                      There are no DAGs matching your current filters. Try
                      adjusting your search criteria or tags.
                    </p>
                    <CreateDAGModal />
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Card View - Visible on mobile or when panel is narrow */}
      <div className={`space-y-2 ${useCardView ? 'block' : 'md:hidden'}`}>
        {instance.getRowModel().rows.length ? (
          instance.getRowModel().rows.map((row) => {
            // Render group rows with collapsible header
            if (row.original?.kind === ItemKind.Group) {
              const groupRow = row.original as GroupRow;
              const isExpanded = row.getIsExpanded();

              return (
                <div key={row.id} className="space-y-1.5">
                  {/* Group Header */}
                  <div
                    className="flex items-center justify-between px-3 py-2 bg-muted/70 rounded-md border border-muted-foreground/20 cursor-pointer active:bg-muted"
                    onClick={() => row.toggleExpanded()}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <div className="flex-shrink-0">
                        {isExpanded ? (
                          <ChevronDown className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <ChevronUp className="h-4 w-4 text-muted-foreground" />
                        )}
                      </div>
                      <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wide truncate">
                        {groupRow.name}
                      </span>
                    </div>
                    <Badge
                      variant="secondary"
                      className="text-[10px] px-1.5 py-0 h-4 flex-shrink-0"
                    >
                      {row.subRows?.length || 0}
                    </Badge>
                  </div>

                  {/* Group Members - only shown when expanded */}
                  {isExpanded && row.subRows && row.subRows.length > 0 && (
                    <div className="space-y-1.5 pl-2 border-l-2 border-muted-foreground/20 ml-3">
                      {row.subRows.map((subRow) => {
                        if (
                          subRow.original?.kind === ItemKind.DAG &&
                          'dag' in subRow.original
                        ) {
                          const dagRow = subRow.original as DAGRow;
                          return (
                            <DAGCard
                              key={subRow.id}
                              dag={dagRow.dag}
                              isSelected={selectedDAG === dagRow.dag.fileName}
                              onSelect={handleSelectDAG}
                              onTagClick={handleSearchTagChange}
                              refreshFn={refreshFn}
                              className="ml-2"
                            />
                          );
                        }
                        return null;
                      })}
                    </div>
                  )}
                </div>
              );
            }

            // Render standalone DAG rows (not in a group)
            // Skip if this row has a parent (it's already rendered within a group)
            if (
              row.original?.kind === ItemKind.DAG &&
              'dag' in row.original &&
              row.depth === 0
            ) {
              const dagRow = row.original as DAGRow;
              return (
                <DAGCard
                  key={row.id}
                  dag={dagRow.dag}
                  isSelected={selectedDAG === dagRow.dag.fileName}
                  onSelect={handleSelectDAG}
                  onTagClick={handleSearchTagChange}
                  refreshFn={refreshFn}
                />
              );
            }

            return null;
          })
        ) : (
          <div className="flex flex-col items-center justify-center py-12 px-4 border rounded-md bg-card">
            <div className="text-6xl mb-4">🔍</div>
            <h3 className="text-lg font-medium mb-2">No DAGs found</h3>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-4">
              There are no DAGs matching your current filters. Try adjusting
              your search criteria or tags.
            </p>
            <CreateDAGModal />
          </div>
        )}
      </div>
    </div>
  );
}

export default DAGTable;
