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
import React, { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import Ticker from '../../../../ui/Ticker';
import VisuallyHidden from '../../../../ui/VisuallyHidden';
import { DAGDetailsModal } from '../../components/dag-details';
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
  parts.push(`${secs}s`);
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
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';

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
      <Button
        variant="ghost"
        size="icon"
        onClick={table.getToggleAllRowsExpandedHandler()}
        className="text-muted-foreground cursor-pointer" // Use Tailwind for color
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
      </Button>
    ),
    cell: ({ row }) => {
      if (row.getCanExpand()) {
        return (
          <Button
            variant="ghost"
            size="icon"
            onClick={row.getToggleExpandedHandler()}
            className="text-muted-foreground cursor-pointer"
          >
            {row.getIsExpanded() ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </Button>
        );
      }
      return null; // Return null instead of empty string for clarity
    },
    size: 40,
    minSize: 40,
    maxSize: 40,
  }),
  columnHelper.accessor('name', {
    id: 'Name',
    size: 350,
    minSize: 200,
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
        // Group Row: Render group name directly
        return (
          <div style={{ paddingLeft: `${row.depth * 1.5}rem` }}>
            <span className="font-normal text-muted-foreground">
              {getValue()}
            </span>{' '}
            {/* Muted color group text */}
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
    size: 100,
    minSize: 100,
    maxSize: 120,
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
    size: 150,
    minSize: 150,
    maxSize: 180,
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
            // Format duration manually without using the custom format function
            const duration = dayjs.duration(durationMs);
            const days = Math.floor(duration.asDays());
            const hours = duration.hours();
            const minutes = duration.minutes();
            const seconds = duration.seconds();

            const parts: string[] = [];
            if (days > 0) parts.push(`${days}d`);
            if (hours > 0) parts.push(`${hours}h`);
            if (minutes > 0) parts.push(`${minutes}m`);
            if (seconds > 0 && parts.length === 0) parts.push(`${seconds}s`);

            const formattedDuration = parts.join(' ');

            durationContent = (
              <div className="text-[10px] text-muted-foreground">
                {formattedDuration}
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
        <div className="space-y-0.5">
          <span className="font-normal text-foreground/70 text-xs">
            {formattedStartedAt}
          </span>
          {durationContent}
        </div>
      );
    },
  }),
  columnHelper.accessor('kind', {
    id: 'ScheduleAndNextRun',
    size: 180,
    minSize: 150,
    maxSize: 200,
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Schedule</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Next execution
        </span>
      </div>
    ),
    cell: ({ row }) => {
      const data = row.original!;
      if (data.kind === ItemKind.DAG) {
        const schedules = data.dag.dag.schedule || [];

        if (schedules.length === 0) {
          return null;
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
          <div className="space-y-0.5">
            {scheduleContent}
            {nextRunContent}
          </div>
        );
      }
      return null;
    },
  }),
  // Description column removed as description is now displayed under the name
  columnHelper.accessor('kind', {
    id: 'Live',
    size: 70,
    minSize: 70,
    maxSize: 70,
    header: () => (
      <div className="flex flex-col py-1">
        <span className="text-xs">Live</span>
        <span className="text-[10px] font-normal text-muted-foreground">
          Auto-schedule
        </span>
      </div>
    ),
    cell: ({ row, table }) => {
      // Use row and table
      const data = row.original!;
      if (data.kind !== ItemKind.DAG) {
        return null; // Changed from false to null
      }
      // Wrap LiveSwitch in a div and stop propagation on its click
      return (
        <div
          onClick={(e) => e.stopPropagation()}
          className="flex justify-center scale-90" // Scale the container instead
        >
          <LiveSwitch
            dag={data.dag}
            refresh={table.options.meta?.refreshFn}
            aria-label={`Toggle ${data.name}`}
          />
        </div>
      );
    },
  }),
  columnHelper.display({
    id: 'Actions',
    size: 100,
    minSize: 100,
    maxSize: 100,
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
}: Props) {
  const navigate = useNavigate();
  const [columns] = React.useState(() => [...defaultColumns]);
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    []
  );
  const [expanded, setExpanded] = React.useState<ExpandedState>({});

  // State for the side modal
  const [selectedDAG, setSelectedDAG] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  // State for client-side sorting
  const [clientSort, setClientSort] = React.useState<string>('');
  const [clientOrder, setClientOrder] = React.useState<string>('asc');

  // Handler for client-side sorting
  const handleClientSort = (field: string, order: string) => {
    setClientSort(field);
    setClientOrder(order);
  };

  // Handlers for the modal
  const openModal = (fileName: string) => {
    // Check if screen is small (less than 768px width)
    const isSmallScreen = window.innerWidth < 768;

    if (isSmallScreen) {
      // For small screens, navigate directly to the DAG details page
      navigate(`/dags/${fileName}`);
    } else {
      // For larger screens, open the side modal
      setSelectedDAG(fileName);
      setIsModalOpen(true);
    }
  };

  const closeModal = () => {
    setIsModalOpen(false);
  };

  // Close modal and navigate to full page on window resize if screen becomes small
  React.useEffect(() => {
    const handleResize = () => {
      if (isModalOpen && selectedDAG && window.innerWidth < 768) {
        // Close the modal
        setIsModalOpen(false);
        // Navigate to the full page
        navigate(`/dags/${selectedDAG}`);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [isModalOpen, selectedDAG, navigate]);

  // Update column filters based on external search props
  React.useEffect(() => {
    const nameFilter = columnFilters.find((f) => f.id === 'Name');
    const tagFilter = columnFilters.find((f) => f.id === 'Tags');

    let updated = false;
    const newFilters = [...columnFilters];

    if (searchText && (!nameFilter || nameFilter.value !== searchText)) {
      const idx = newFilters.findIndex((f) => f.id === 'Name');
      if (idx > -1) newFilters[idx] = { id: 'Name', value: searchText };
      else newFilters.push({ id: 'Name', value: searchText });
      updated = true;
    } else if (!searchText && nameFilter) {
      const idx = newFilters.findIndex((f) => f.id === 'Name');
      if (idx > -1) newFilters.splice(idx, 1);
      updated = true;
    }

    if (searchTag && (!tagFilter || tagFilter.value !== searchTag)) {
      const idx = newFilters.findIndex((f) => f.id === 'Tags');
      if (idx > -1) newFilters[idx] = { id: 'Tags', value: searchTag };
      else newFilters.push({ id: 'Tags', value: searchTag });
      updated = true;
    } else if (!searchTag && tagFilter) {
      const idx = newFilters.findIndex((f) => f.id === 'Tags');
      if (idx > -1) newFilters.splice(idx, 1);
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

  // Add keyboard navigation between DAGs when modal is open
  // Create a ref to store the table instance
  const tableInstanceRef = React.useRef<ReturnType<
    typeof useReactTable
  > | null>(null);

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isModalOpen || !selectedDAG || !tableInstanceRef.current) return;

      // Get all DAG rows from the sorted table rows (not groups)
      const sortedRows = tableInstanceRef.current.getRowModel().rows;
      const dagRows = sortedRows
        .filter((row) => (row.original as Data)?.kind === ItemKind.DAG)
        .map((row) => ({
          fileName: (row.original as DAGRow).dag.fileName,
          row: row.original as DAGRow,
        }));

      // Find current index
      const currentIndex = dagRows.findIndex(
        (item) => item.fileName === selectedDAG
      );
      if (currentIndex === -1) return;

      // Navigate with arrow keys
      if (event.key === 'ArrowDown' && currentIndex < dagRows.length - 1) {
        // Move to next DAG
        const nextDAG = dagRows[currentIndex + 1];
        if (nextDAG) {
          setSelectedDAG(nextDAG.fileName);
        }
      } else if (event.key === 'ArrowUp' && currentIndex > 0) {
        // Move to previous DAG
        const prevDAG = dagRows[currentIndex - 1];
        if (prevDAG) {
          setSelectedDAG(prevDAG.fileName);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isModalOpen, selectedDAG]); // No need for sorting in dependencies anymore

  const instance = useReactTable<Data>({
    data,
    columns,
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
    onExpandedChange: setExpanded,
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
  const { data: uniqueTags } = useQuery('/dags/tags', {
    params: {
      query: {
        remoteNode: appBarContext?.selectedRemoteNode || 'local',
      },
    },
  });

  return (
    <div className="space-y-4">
      {/* Side Modal for DAG Details */}
      {selectedDAG && (
        <DAGDetailsModal
          fileName={selectedDAG}
          isOpen={isModalOpen}
          onClose={closeModal}
        />
      )}

      {/* Search, Filter and Pagination Controls */}
      <div
        className={`bg-muted/50 rounded-lg p-3 mb-4 space-y-3 ${
          isLoading ? 'opacity-70 pointer-events-none' : ''
        }`}
      >
        <div className="flex flex-col sm:flex-row flex-wrap items-stretch sm:items-center gap-2">
          {/* Search and Filter Row */}
          <div className="flex flex-1 gap-2 min-w-0">
            {/* Search input */}
            <div className="relative flex-1 min-w-0">
              <div className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                <Search className="h-4 w-4" />
              </div>
              <Input
                type="search"
                placeholder="Search definitions..."
                value={searchText}
                onChange={(e) => handleSearchTextChange(e.target.value)}
                className="pl-10 h-9 border border-input rounded-md w-full"
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
              <SelectTrigger className="w-auto min-w-[120px] sm:min-w-[160px] h-9 border border-input rounded-md">
                <div className="flex items-center gap-2">
                  <Filter className="h-4 w-4 text-muted-foreground" />
                  <SelectValue placeholder="Filter by tag" />
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
          </div>

          {/* Pagination - on new row on mobile */}
          {pagination && (
            <div className="flex justify-center sm:justify-end sm:ml-auto">
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

      {/* Desktop Table View - Hidden on mobile */}
      <div
        className="hidden md:block rounded-xl w-full max-w-full min-w-0 overflow-x-auto transition-all duration-300"
        style={{
          fontFamily:
            'ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"',
          background:
            'linear-gradient(to bottom, var(--background) 0%, var(--background) 100%)',
          border: '1px solid var(--border)',
          borderRadius: '0.75rem',
        }}
      >
        <Table
          className={`w-full text-xs table-fixed ${isLoading ? 'opacity-70' : ''}`}
        >
          <TableHeader>
            {instance.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={
                      'py-1 px-2 ' +
                      (header.column.id === 'Description'
                        ? 'max-w-[250px] '
                        : '') +
                      'text-muted-foreground text-xs'
                    }
                    style={{
                      width: header.getSize(),
                      minWidth: header.column.columnDef.minSize,
                      maxWidth: header.column.columnDef.maxSize,
                      fontWeight: 500, // Medium weight headers
                      fontSize: '0.75rem', // Smaller font size for headers
                    }}
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
                            children={flexRender(
                              header.column.columnDef.header,
                              header.getContext()
                            )}
                          />
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
                    className={
                      row.original?.kind === ItemKind.Group
                        ? 'bg-muted/50 font-semibold cursor-pointer hover:bg-muted/70' // Make group rows clickable
                        : isDAGRow &&
                            'dag' in row.original &&
                            selectedDAG ===
                              (row.original as DAGRow).dag.fileName
                          ? 'cursor-pointer bg-primary/10 hover:bg-primary/15 border-l-4 border-primary border-b-0' // Highlight selected DAG
                          : 'cursor-pointer hover:bg-muted/50'
                    }
                    style={{ fontSize: '0.8125rem' }} // Smaller font size for more density
                    onClick={(e) => {
                      // Handle group row clicks - toggle expanded state
                      if ((row.original as Data)?.kind === ItemKind.Group) {
                        row.toggleExpanded();
                      }
                      // Handle DAG row clicks - open modal or new tab
                      else if (isDAGRow && 'dag' in row.original) {
                        const dagRow = row.original as DAGRow;
                        const fileName = dagRow.dag.fileName;

                        // If Cmd (Mac) or Ctrl (Windows/Linux) key is pressed, open in new tab
                        if (e.metaKey || e.ctrlKey) {
                          // Open in new tab
                          window.open(`/dags/${fileName}`, '_blank');
                        } else {
                          // Normal click behavior
                          openModal(fileName);
                        }
                      }
                    }}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        className="py-1 px-2 overflow-hidden"
                        style={{
                          width: cell.column.getSize(),
                          minWidth: cell.column.columnDef.minSize,
                          maxWidth: cell.column.columnDef.maxSize,
                        }}
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

      {/* Mobile Card View - Visible only on mobile */}
      <div className="md:hidden space-y-2">
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
                          const dag = dagRow.dag;
                          const fileName = dag.fileName;
                          const status = dag.latestDAGRun.status;
                          const statusLabel = dag.latestDAGRun.statusLabel;
                          const tags = dag.dag.tags || [];
                          const description = dag.dag.description;

                          return (
                            <div
                              key={subRow.id}
                              className={`p-2.5 ml-2 rounded-md border ${
                                selectedDAG === fileName
                                  ? 'bg-primary/10 border-primary'
                                  : 'bg-card border-border'
                              } cursor-pointer active:scale-[0.98] transition-transform`}
                              onClick={(e) => {
                                if (e.metaKey || e.ctrlKey) {
                                  window.open(`/dags/${fileName}`, '_blank');
                                } else {
                                  openModal(fileName);
                                }
                              }}
                            >
                              {/* Compact header */}
                              <div className="flex justify-between items-start gap-2 mb-1.5">
                                <div className="font-medium text-xs truncate flex-1 min-w-0">
                                  {dag.dag.name}
                                </div>
                                <StatusChip status={status} size="xs">
                                  {statusLabel}
                                </StatusChip>
                              </div>

                              {/* Description - compact */}
                              {description && (
                                <div className="text-[10px] text-muted-foreground mb-1.5 line-clamp-1">
                                  {description}
                                </div>
                              )}

                              {/* Schedule & Last Run - inline */}
                              <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground mb-1.5">
                                {dag.dag.schedule && dag.dag.schedule.length > 0
                                  ? dag.dag.schedule.map((schedule, idx) => (
                                      <Badge
                                        key={idx}
                                        variant="outline"
                                        className="text-[9px] font-normal px-1 py-0 h-3"
                                      >
                                        {schedule.expression}
                                      </Badge>
                                    ))
                                  : null}
                                {dag.latestDAGRun.startedAt &&
                                  dag.latestDAGRun.startedAt !== '-' && (
                                    <span className="flex items-center gap-0.5">
                                      <Calendar className="h-2.5 w-2.5" />
                                      <span className="text-[9px]">
                                        {dag.latestDAGRun.startedAt}
                                      </span>
                                    </span>
                                  )}
                              </div>

                              {/* Tags - compact */}
                              {tags.length > 0 && (
                                <div className="flex flex-wrap gap-0.5">
                                  {tags.map((tag) => (
                                    <Badge
                                      key={tag}
                                      variant="outline"
                                      className="text-[9px] px-1 py-0 h-3 rounded-sm border-primary/30 bg-primary/10 text-primary"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        handleSearchTagChange(tag);
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
              const dag = dagRow.dag;
              const fileName = dag.fileName;
              const status = dag.latestDAGRun.status;
              const statusLabel = dag.latestDAGRun.statusLabel;
              const tags = dag.dag.tags || [];
              const description = dag.dag.description;

              return (
                <div
                  key={row.id}
                  className={`p-2.5 rounded-md border ${
                    selectedDAG === fileName
                      ? 'bg-primary/10 border-primary'
                      : 'bg-card border-border'
                  } cursor-pointer active:scale-[0.98] transition-transform`}
                  onClick={(e) => {
                    if (e.metaKey || e.ctrlKey) {
                      window.open(`/dags/${fileName}`, '_blank');
                    } else {
                      openModal(fileName);
                    }
                  }}
                >
                  {/* Compact header */}
                  <div className="flex justify-between items-start gap-2 mb-1.5">
                    <div className="font-medium text-xs truncate flex-1 min-w-0">
                      {dag.dag.name}
                    </div>
                    <StatusChip status={status} size="xs">
                      {statusLabel}
                    </StatusChip>
                  </div>

                  {/* Description - compact */}
                  {description && (
                    <div className="text-[10px] text-muted-foreground mb-1.5 line-clamp-1">
                      {description}
                    </div>
                  )}

                  {/* Schedule & Last Run - inline */}
                  <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground mb-1.5">
                    {dag.dag.schedule && dag.dag.schedule.length > 0
                      ? dag.dag.schedule.map((schedule, idx) => (
                          <Badge
                            key={idx}
                            variant="outline"
                            className="text-[9px] font-normal px-1 py-0 h-3"
                          >
                            {schedule.expression}
                          </Badge>
                        ))
                      : null}
                    {dag.latestDAGRun.startedAt &&
                      dag.latestDAGRun.startedAt !== '-' && (
                        <span className="flex items-center gap-0.5">
                          <Calendar className="h-2.5 w-2.5" />
                          <span className="text-[9px]">
                            {dag.latestDAGRun.startedAt}
                          </span>
                        </span>
                      )}
                  </div>

                  {/* Tags - compact */}
                  {tags.length > 0 && (
                    <div className="flex flex-wrap gap-0.5">
                      {tags.map((tag) => (
                        <Badge
                          key={tag}
                          variant="outline"
                          className="text-[9px] px-1 py-0 h-3 rounded-sm border-primary/30 bg-primary/10 text-primary"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleSearchTagChange(tag);
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
