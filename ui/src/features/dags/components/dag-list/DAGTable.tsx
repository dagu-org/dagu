import React, { useMemo } from 'react';
import {
  flexRender,
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  getFilteredRowModel,
  ColumnFiltersState,
  ExpandedState,
  getExpandedRowModel,
  createColumnHelper,
  RowData,
  Column, // Import Column type
} from '@tanstack/react-table';
import cronParser, { CronDate } from 'cron-parser';
import DAGActions from '../common/DAGActions';
import StatusChip from '../../../../ui/StatusChip'; // Re-add StatusChip import
import { Link } from 'react-router-dom';
import {
  ArrowDown, // Use lucide-react icons
  ArrowUp,
  ChevronDown,
  ChevronUp,
  Search, // Icon for search input
  Filter, // Icon for filter button (if needed later)
} from 'lucide-react';
import LiveSwitch from '../common/LiveSwitch';
import 'moment-duration-format';
import Ticker from '../../../../ui/Ticker';
import VisuallyHidden from '../../../../ui/VisuallyHidden';
import moment from 'moment-timezone';
import { components } from '../../../../api/v2/schema';

// Import shadcn/ui components
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'; // Use shadcn Select

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
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface TableMeta<TData extends RowData> {
    group: string;
    refreshFn: () => void;
    // Add tag change handler to meta for direct access in cell
    handleSearchTagChange?: (tag: string) => void;
  }
}

const columnHelper = createColumnHelper<Data>();

// --- Helper Functions (moved from bottom for clarity) ---
function getConfig() {
  // Assuming getConfig is defined elsewhere or replace with actual config access
  return { tz: moment.tz.guess() };
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
      const options = {
        tz: getConfig().tz,
        iterator: true,
      };
      // Assuming 'parseExpression' is the correct method name based on library docs
      const interval = cronParser.parse(schedule.expression, options);
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

function getFirstTag(data?: Data): string {
  // Explicitly check array existence and non-emptiness
  const tags = data?.kind === ItemKind.DAG ? data.dag?.dag?.tags : undefined;
  if (tags && tags.length > 0 && typeof tags[0] === 'string') {
    // Now tags[0] is confirmed to be a string
    return tags[0].toLowerCase();
  }
  return '';
}

// Allow returning number for group sorting placeholder
function getStatus(data: RowItem): components['schemas']['Status'] | number {
  if (data.kind === ItemKind.DAG) {
    return data.dag.latestRun.status;
  }
  // Use a number outside the Status enum range for groups
  return -1;
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
        className="text-muted-foreground" // Use Tailwind for color
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
            className="text-muted-foreground"
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
    enableSorting: false,
    size: 40, // Example size adjustment
  }),
  columnHelper.accessor('name', {
    id: 'Name',
    header: 'Name', // Simple header text
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
            className="space-y-1"
          >
            <div className="font-medium text-gray-800 dark:text-gray-200 tracking-tight">
              {' '}
              {/* Medium weight, darker color name */}
              {getValue()}
            </div>

            {description && (
              <div className="text-xs text-muted-foreground mt-0.5 whitespace-normal">
                {' '}
                {/* Allow wrapping */} {/* Keep description small */}
                {description}
              </div>
            )}

            {tags.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-1.5">
                {' '}
                {/* Adjust tag spacing */}
                {tags.map((tag) => (
                  <Badge
                    key={tag}
                    variant="secondary"
                    className="text-xs px-1.5 py-0.5 cursor-pointer hover:bg-muted font-normal"
                    onClick={(e) => {
                      e.stopPropagation(); // Prevent row click
                      e.preventDefault();
                      // Get the handleSearchTagChange from the component props
                      const handleTagClick =
                        table.options.meta?.handleSearchTagChange;
                      if (handleTagClick) handleTagClick(tag);
                    }}
                  >
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
        const description = (data.dag.dag.description || '').toLowerCase();
        const searchValue = String(filterValue).toLowerCase();

        // Search in name and description
        if (name.includes(searchValue) || description.includes(searchValue)) {
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
    sortingFn: (a, b) => {
      const ta = a.original!.kind;
      const tb = b.original!.kind;
      if (ta === tb) {
        const nameA = a.original!.name.toLowerCase();
        const nameB = b.original!.name.toLowerCase();
        return nameA.localeCompare(nameB);
      }
      // Keep groups potentially sorted differently if needed, or simply by name
      return ta === ItemKind.Group ? -1 : 1; // Example: Groups first
    },
  }),
  // Tags column removed as tags are now displayed under the name
  // The filter functionality is preserved in the Name column
  columnHelper.accessor('kind', {
    id: 'Status',
    header: () => (
      <div className="flex flex-col py-4">
        <span>Status</span>
        <span className="text-xs font-normal text-muted-foreground mt-0.5">
          Current state
        </span>
      </div>
    ),
    cell: ({ row }) => {
      // Use row
      const data = row.original!;
      if (data.kind === ItemKind.DAG) {
        // Use the updated StatusChip component
        return (
          <StatusChip status={data.dag.latestRun.status}>
            {data.dag.latestRun?.statusText}
          </StatusChip>
        );
      }
      return null;
    },
    sortingFn: (a, b) => {
      // Explicitly handle number type for comparison
      const valA = getStatus(a.original) as number;
      const valB = getStatus(b.original) as number;
      return valA - valB;
    },
  }),
  // Removed Started At and Finished At columns
  columnHelper.accessor('kind', {
    id: 'LastRun',
    header: () => (
      <div className="flex flex-col py-4">
        <span>Last Run</span>
        <span className="text-xs font-normal text-muted-foreground mt-0.5">
          {getConfig().tz || 'Local Timezone'}
        </span>
      </div>
    ),
    cell: ({ row }) => {
      const data = row.original!;
      if (data.kind !== ItemKind.DAG) {
        return null;
      }

      const { startedAt, finishedAt, status } = data.dag.latestRun;

      if (!startedAt || startedAt === '-') {
        // If no start time, display nothing or a placeholder
        return <span className="font-normal text-muted-foreground">-</span>;
      }

      const formattedStartedAt = moment(startedAt).format(
        'YYYY-MM-DD HH:mm:ss'
      );
      let durationContent = null;

      if (finishedAt && finishedAt !== '-') {
        const durationMs = moment(finishedAt).diff(moment(startedAt));
        // Choose format based on duration length
        const format = durationMs >= 1000 * 60 ? 'd[d] h[h] m[m]' : 's[s]';
        const formattedDuration = moment.duration(durationMs).format(format);
        if (durationMs > 0) {
          // Only show duration if positive
          durationContent = (
            <div className="text-xs text-muted-foreground mt-0.5">
              Duration: {formattedDuration}
            </div>
          );
        }
      } else if (status === 1) {
        // Status 1 typically means "Running"
        durationContent = (
          <div className="text-xs text-muted-foreground mt-0.5">(Running)</div>
        );
      }

      return (
        <div>
          <span className="font-normal text-gray-700 dark:text-gray-300">
            {' '}
            {/* Match DAG name color */}
            {formattedStartedAt}
          </span>
          {durationContent}
        </div>
      );
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind !== ItemKind.DAG || dataB.kind !== ItemKind.DAG) {
        // Handle sorting for non-DAG rows if necessary, e.g., groups first
        return dataA.kind === ItemKind.Group ? -1 : 1;
      }
      // Prioritize rows with startedAt dates
      const startedAtA = dataA.dag.latestRun.startedAt;
      const startedAtB = dataB.dag.latestRun.startedAt;

      if (!startedAtA && !startedAtB) return 0; // Both null/undefined
      if (!startedAtA) return 1; // A is null, should come after B
      if (!startedAtB) return -1; // B is null, should come after A

      // Compare valid dates using moment's diff for accurate comparison
      return moment(startedAtA).diff(moment(startedAtB));
    },
    size: 200, // Adjust size as needed
  }),
  columnHelper.accessor('kind', {
    id: 'ScheduleAndNextRun',
    header: () => (
      <div className="flex flex-col">
        <span>Schedule</span>
        <span className="text-xs font-normal text-muted-foreground mt-0.5">
          Next execution
        </span>
      </div>
    ),
    enableSorting: true,
    cell: ({ row }) => {
      const data = row.original!;
      if (data.kind === ItemKind.DAG) {
        const schedules = data.dag.dag.schedule || [];

        if (schedules.length === 0) {
          return null;
        }

        // Display schedule expressions
        const scheduleContent = (
          <div className="flex flex-wrap gap-1 mb-1.5">
            {schedules.map((schedule) => (
              <Badge
                key={schedule.expression}
                variant="outline"
                className="text-xs font-normal px-1.5 py-0.5"
              >
                {schedule.expression}
              </Badge>
            ))}
          </div>
        );

        // Display next run information
        let nextRunContent = null;
        if (!data.dag.suspended && schedules.length > 0) {
          const nextRun = getNextSchedule(data.dag);
          if (nextRun) {
            nextRunContent = (
              <div className="text-xs text-muted-foreground font-normal">
                <Ticker intervalMs={1000}>
                  {() => {
                    const ms = nextRun.getTime() - new Date().getTime();
                    const durFormat =
                      ms > 1000 * 60 * 60 ? 'd[d]h[h]m[m]' : 'd[d]h[h]m[m]s[s]';
                    return (
                      <span>
                        Run in {moment.duration(ms).format(durFormat)}
                      </span>
                    );
                  }}
                </Ticker>
              </div>
            );
          }
        } else if (data.dag.suspended) {
          nextRunContent = (
            <div className="text-xs text-muted-foreground font-normal">
              Suspended
            </div>
          );
        }

        return (
          <div>
            {scheduleContent}
            {nextRunContent}
          </div>
        );
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind !== ItemKind.DAG || dataB.kind !== ItemKind.DAG) {
        return dataA!.kind - dataB!.kind;
      }
      const nextA = getNextSchedule(dataA.dag);
      const nextB = getNextSchedule(dataB.dag);
      if (!nextA && !nextB) {
        return 0; // Both are undefined
      }
      if (!nextA) {
        return 1; // A is undefined, B is defined
      }
      if (!nextB) {
        return -1; // B is undefined, A is defined
      }
      return nextA.getTime() - nextB.getTime();
    },
  }),
  // Description column removed as description is now displayed under the name
  columnHelper.accessor('kind', {
    id: 'Live',
    header: () => (
      <div className="flex flex-col">
        <span>Live</span>
        <span className="text-xs font-normal text-muted-foreground mt-0.5">
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
          className="flex justify-center"
        >
          <LiveSwitch
            dag={data.dag}
            refresh={table.options.meta?.refreshFn}
            aria-label={`Toggle ${data.name}`} // Pass aria-label directly
          />
        </div>
      );
    },
    size: 60, // Example size
  }),
  columnHelper.display({
    id: 'Actions',
    header: () => (
      <div className="flex flex-col items-center">
        <span>Actions</span>
        <span className="text-xs font-normal text-muted-foreground mt-0.5">
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
          className="flex justify-center"
          onClick={(e) => e.stopPropagation()}
        >
          <DAGActions
            dag={data.dag.dag}
            status={data.dag.latestRun}
            fileId={data.dag.fileId}
            label={false}
            refresh={table.options.meta?.refreshFn}
          />
        </div>
      );
    },
    size: 100, // Example size
  }),
];

// --- Header Component for Sorting ---
const SortableHeader = ({
  column,
  children,
}: {
  column: Column<Data, unknown>;
  children: React.ReactNode;
}) => {
  const sort = column.getIsSorted();
  return (
    <Button
      variant="ghost"
      onClick={column.getToggleSortingHandler()}
      className="-ml-4 h-8" // Adjust spacing
    >
      {children}
      {sort === 'asc' && <ArrowUp className="ml-2 h-4 w-4" />}
      {sort === 'desc' && <ArrowDown className="ml-2 h-4 w-4" />}
    </Button>
  );
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
}: Props) {
  const [columns] = React.useState(() => [...defaultColumns]);
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    []
  );
  const [sorting, setSorting] = React.useState<SortingState>([
    { id: 'Name', desc: false },
  ]);
  const [expanded, setExpanded] = React.useState<ExpandedState>({});

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
    const groups: { [key: string]: Data } = {};
    dags.forEach((dag) => {
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
    const hierarchicalData: Data[] = Object.values(groups); // Get group objects
    // Add DAGs without a group
    dags
      .filter((dag) => !dag.dag.group)
      .forEach((dag) => {
        hierarchicalData.push({
          kind: ItemKind.DAG,
          name: dag.dag.name,
          dag: dag,
        });
      });
    return hierarchicalData;
  }, [dags]); // Removed 'group' dependency as it's handled by filtering

  const instance = useReactTable<Data>({
    data,
    columns,
    getSubRows: (row) => row.subRows,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onColumnFiltersChange: setColumnFilters, // Let table manage internal filter state
    getExpandedRowModel: getExpandedRowModel(),
    autoResetExpanded: false, // Keep expanded state on data change
    state: {
      sorting,
      expanded,
      columnFilters, // Pass filters to table state
    },
    onSortingChange: setSorting,
    onExpandedChange: setExpanded,
    // Pass handlers via meta
    meta: {
      group, // Pass group if needed elsewhere
      refreshFn,
      handleSearchTagChange, // Pass tag handler
    },
  });

  // Extract unique tags for the Select dropdown
  const uniqueTags = useMemo(() => {
    const tagsSet = new Set<string>();
    dags.forEach((dag) => {
      dag.dag.tags?.forEach((tag) => tagsSet.add(tag));
    });
    return Array.from(tagsSet).sort();
  }, [dags]);

  return (
    <div className="space-y-4">
      {' '}
      {/* Add spacing */}
      {/* Filter Controls */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-full sm:w-[260px]">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            type="search"
            placeholder="Search DAGs by name..."
            value={searchText}
            onChange={(e) => handleSearchTextChange(e.target.value)}
            className="pl-8 w-full"
          />
        </div>
        <Select
          value={searchTag}
          onValueChange={(value) =>
            handleSearchTagChange(value === 'all' ? '' : value)
          } // Handle 'all' value
        >
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="Filter by tag" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Tags</SelectItem>
            {uniqueTags.map((tag) => (
              <SelectItem key={tag} value={tag}>
                {tag}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {/* Add other filters/buttons here if needed */}
      </div>
      {/* Table */}
      {/* Add overflow-x-auto, max-w-full, min-w-0, shadow, and padding for card look */}
      <div
        className="rounded-xl border bg-card w-full max-w-full min-w-0 shadow-sm overflow-x-auto"
        style={{
          fontFamily:
            'ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"',
        }}
      >
        <Table className="w-full">
          <TableHeader>
            {instance.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={
                      'py-1 ' +
                      (header.column.id === 'Description'
                        ? 'max-w-[250px] '
                        : '') +
                      'text-muted-foreground'
                    }
                    style={{
                      width:
                        header.getSize() !== 150 ? header.getSize() : undefined,
                      maxWidth:
                        header.column.id === 'Description'
                          ? '250px'
                          : undefined,
                      fontWeight: 500, // Medium weight headers
                      fontSize: '0.875rem',
                    }}
                  >
                    {header.isPlaceholder ? null : (
                      <div>
                        {' '}
                        {/* Wrap header content */}
                        {header.column.getCanSort() ? (
                          <SortableHeader column={header.column}>
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
                const navigateTo =
                  isDAGRow && 'dag' in row.original
                    ? `/dags/${(row.original as DAGRow).dag.fileId}`
                    : undefined;

                return (
                  <TableRow
                    key={row.id}
                    data-state={row.getIsSelected() && 'selected'}
                    className={
                      row.original?.kind === ItemKind.Group
                        ? 'bg-muted/50 font-semibold' // Keep group rows semi-bold
                        : 'cursor-pointer hover:bg-muted/50'
                    }
                    style={{ fontSize: '0.9375rem' }} // Ensure row font size matches container
                    onClick={() => {
                      if (isDAGRow && navigateTo) {
                        window.location.href = navigateTo;
                      }
                    }}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        style={{
                          maxWidth:
                            cell.column.id === 'Name' ? '350px' : undefined, // Apply max-width to Name cell
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
                  className="h-24 text-center"
                >
                  No results.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

export default DAGTable;
