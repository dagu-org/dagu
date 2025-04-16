/**
 * DAGTable component displays a table of DAGs with filtering, sorting, and grouping capabilities.
 *
 * @module features/dags/components/dag-list
 */
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
} from '@tanstack/react-table';
import DAGActions from '../common/DAGActions';
import StatusChip from '../../../../ui/StatusChip';
import {
  Autocomplete,
  Box,
  Chip,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
} from '@mui/material';
import { Link } from 'react-router-dom';
import { getNextSchedule } from '../../../../models';
import StyledTableRow from '../../../../ui/StyledTableRow';
import {
  ArrowDownward,
  ArrowUpward,
  KeyboardArrowDown,
  KeyboardArrowUp,
} from '@mui/icons-material';
import LiveSwitch from '../common/LiveSwitch';
import 'moment-duration-format';
import Ticker from '../../../../ui/Ticker';
import VisuallyHidden from '../../../../ui/VisuallyHidden';
import moment from 'moment-timezone';
import { components } from '../../../../api/v2/schema';

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

const durFormatSec = 'd[d]h[h]m[m]s[s]';
const durFormatMin = 'd[d]h[h]m[m]';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface TableMeta<TData extends RowData> {
    group: string;
    refreshFn: () => void;
  }
}

const columnHelper = createColumnHelper<Data>();

const defaultColumns = [
  columnHelper.accessor('name', {
    id: 'Expand',
    header: ({ table }) => {
      return (
        <IconButton
          onClick={table.getToggleAllRowsExpandedHandler()}
          className="gray-90"
        >
          {table.getIsAllRowsExpanded() ? (
            <>
              <VisuallyHidden>Compress rows</VisuallyHidden>
              <KeyboardArrowUp />
            </>
          ) : (
            <>
              <VisuallyHidden>Expand rows</VisuallyHidden>
              <KeyboardArrowDown />
            </>
          )}
        </IconButton>
      );
    },
    cell: ({ row }) => {
      if (row.getCanExpand()) {
        return (
          <IconButton onClick={row.getToggleExpandedHandler()}>
            {row.getIsExpanded() ? <KeyboardArrowUp /> : <KeyboardArrowDown />}
          </IconButton>
        );
      }
      return '';
    },
    enableSorting: false,
  }),
  columnHelper.accessor('name', {
    id: 'Name',
    cell: ({ row, getValue }) => {
      const data = row.original!;
      if (data.kind == ItemKind.Group) {
        return getValue();
      } else {
        const url = `/dags/${data.dag.fileId}`;
        return (
          <div
            style={{
              paddingLeft: `${row.depth * 2}rem`,
            }}
          >
            <Link to={url}>{getValue()}</Link>
          </div>
        );
      }
    },
    filterFn: (props, _, filter) => {
      const data = props.original!;
      if (data.kind == ItemKind.Group) {
        return true;
      } else if (data.kind == ItemKind.DAG) {
        const value = data.dag.dag.name;
        return value.toLowerCase().includes(filter.toLowerCase());
      }
      return false;
    },
    sortingFn: (a, b) => {
      const ta = a.original!.kind;
      const tb = b.original!.kind;
      if (ta == tb) {
        const dataA = a.original!.name.toLowerCase();
        const dataB = b.original!.name.toLowerCase();
        return dataA.localeCompare(dataB);
      }
      if (ta == ItemKind.Group) {
        return 1;
      }
      return -1;
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Tags',
    header: 'Tags',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        const tags = data.dag.dag.tags;
        return (
          <Stack direction="row" spacing={1}>
            {tags?.map((tag) => (
              <Chip
                key={tag}
                size="small"
                label={tag}
                onClick={() => {
                  props.column.setFilterValue(tag);
                }}
              />
            ))}
          </Stack>
        );
      }
      return null;
    },
    filterFn: (props, _, filter) => {
      const data = props.original!;
      if (data.kind != ItemKind.DAG) {
        return true;
      }
      const tags = data.dag.dag.tags;
      const ret = tags?.some((tag) => tag == filter) || false;
      return ret;
    },
    sortingFn: (a, b) => {
      const valA = getFirstTag(a.original);
      const valB = getFirstTag(b.original);
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Status',
    header: 'Status',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        return (
          <StatusChip status={data.dag.latestRun.status}>
            {data.dag.latestRun?.statusText}
          </StatusChip>
        );
      }
      return null;
    },
    sortingFn: (a, b) => {
      const valA = getStatus(a.original);
      const valB = getStatus(b.original);
      return valA < valB ? -1 : 1;
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Started At',
    header: 'Started At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        return data.dag.latestRun.startedAt;
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind != ItemKind.DAG || dataB.kind != ItemKind.DAG) {
        return 0;
      }
      const valA = dataA.dag.latestRun.startedAt || '';
      const valB = dataB.dag.latestRun.startedAt || '';
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Finished At',
    header: 'Finished At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        return data.dag.latestRun.finishedAt;
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind != ItemKind.DAG || dataB.kind != ItemKind.DAG) {
        return 0;
      }
      const valA = dataA.dag.latestRun.finishedAt || '';
      const valB = dataB.dag.latestRun.finishedAt || '';
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Schedule',
    header: `Schedule in ${getConfig().tz || moment.tz.guess()}`,
    enableSorting: true,
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        const schedules = data.dag.dag.schedule;
        if (schedules) {
          return (
            <React.Fragment>
              {schedules.map((schedule) => (
                <Chip
                  key={schedule.expression}
                  sx={{
                    fontWeight: 'semibold',
                    marginRight: 1,
                  }}
                  size="small"
                  label={schedule.expression}
                />
              ))}
            </React.Fragment>
          );
        }
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind != ItemKind.DAG || dataB.kind != ItemKind.DAG) {
        return dataA!.kind - dataB!.kind;
      }
      return getNextSchedule(dataA.dag) - getNextSchedule(dataB.dag);
    },
  }),
  columnHelper.accessor('kind', {
    id: 'NextRun',
    header: 'Next Run',
    enableSorting: true,
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        const schedules = data.dag.dag.schedule;
        if (schedules && schedules.length && !data.dag.suspended) {
          return (
            <React.Fragment>
              in{' '}
              <Ticker intervalMs={1000}>
                {() => {
                  const ms = moment
                    .unix(getNextSchedule(data.dag))
                    .diff(moment.now());
                  const format = ms / 1000 > 60 ? durFormatMin : durFormatSec;
                  return (
                    <span>
                      {moment
                        .duration(ms)
                        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                        // @ts-ignore
                        .format(format)}
                    </span>
                  );
                }}
              </Ticker>
            </React.Fragment>
          );
        }
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      if (dataA.kind != ItemKind.DAG || dataB.kind != ItemKind.DAG) {
        return dataA!.kind - dataB!.kind;
      }
      return getNextSchedule(dataA.dag) - getNextSchedule(dataB.dag);
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Config',
    header: 'Description',
    enableSorting: false,
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.DAG) {
        return data.dag.dag.description;
      }
      return null;
    },
  }),
  columnHelper.accessor('kind', {
    id: 'Live',
    header: 'Live',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind != ItemKind.DAG) {
        return false;
      }
      return (
        <LiveSwitch
          dag={data.dag}
          refresh={props.table.options.meta?.refreshFn}
          inputProps={{
            'aria-label': `Toggle ${data.name}`,
          }}
        />
      );
    },
  }),
  columnHelper.display({
    id: 'Actions',
    header: 'Actions',
    cell: (props) => {
      const data = props.row.original!;
      if (data.kind == ItemKind.Group) {
        return null;
      }

      return (
        <DAGActions
          dag={data.dag.dag}
          status={data.dag.latestRun}
          fileId={data.dag.fileId}
          label={false}
          refresh={props.table.options.meta?.refreshFn}
        />
      );
    },
  }),
];

/**
 * DAGTable component displays a table of DAGs with filtering, sorting, and grouping capabilities
 */
function DAGTable({
  dags = [],
  group = '',
  refreshFn,
  searchText,
  handleSearchTextChange,
  searchTag,
  handleSearchTagChange,
}: Props) {
  const [columns] = React.useState<typeof defaultColumns>(() => [
    ...defaultColumns,
  ]);

  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    []
  );
  const [sorting, setSorting] = React.useState<SortingState>([
    {
      id: 'Name',
      desc: false,
    },
  ]);

  const [expanded, setExpanded] = React.useState<ExpandedState>({});

  // Transform the flat list of DAGs into a hierarchical structure with groups
  const data = useMemo(() => {
    const groups: { [key: string]: Data } = {};
    dags.forEach((dag) => {
      const group = dag.dag.group;
      if (group) {
        if (!groups[group]) {
          groups[group] = {
            kind: ItemKind.Group,
            name: group,
            subRows: [],
          };
        }
        groups[group].subRows!.push({
          kind: ItemKind.DAG,
          name: dag.dag.name,
          dag: dag,
        });
      }
    });
    const data: Data[] = [];
    const groupKeys = Object.keys(groups);
    groupKeys.forEach((key) => {
      if (groups[key]) {
        data.push(groups[key]);
      }
    });
    dags
      .filter((dag) => !dag.dag.group)
      .forEach((dag) => {
        data.push({
          kind: ItemKind.DAG,
          name: dag.dag.name,
          dag: dag,
        });
      });
    return data;
  }, [dags, group]);

  const instance = useReactTable<Data>({
    data,
    columns,
    getSubRows: (row) => row.subRows,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onColumnFiltersChange: setColumnFilters,
    getExpandedRowModel: getExpandedRowModel(),
    autoResetExpanded: false,
    state: {
      sorting,
      expanded,
      columnFilters,
    },
    onSortingChange: setSorting,
    onExpandedChange: setExpanded,
    debugAll: false,
    meta: {
      group,
      refreshFn,
    },
  });

  React.useEffect(() => {
    instance.toggleAllRowsExpanded(true);
  }, []);

  return (
    <Box>
      <Stack
        sx={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'start',
          alignContent: 'flex-center',
        }}
      >
        <TextField
          label="Search Text"
          size="small"
          variant="filled"
          slotProps={{
            htmlInput: {
              value: searchText,
              onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
                handleSearchTextChange(e.target.value);
              },
            },
          }}
          sx={{ marginRight: 2 }}
        />
        <Autocomplete<string, false, false, true>
          freeSolo
          size="small"
          sx={{ width: 200 }}
          options={dags.reduce<string[]>((acc, dag) => {
            const tags = dag.dag.tags;
            if (tags) {
              tags.forEach((tag) => {
                if (!acc.includes(tag)) {
                  acc.push(tag);
                }
              });
            }
            return acc;
          }, [])}
          onChange={(_, value) => {
            handleSearchTagChange(value || '');
          }}
          value={searchTag}
          renderInput={(params) => (
            <TextField {...params} label="Tag" variant="filled" />
          )}
        />
      </Stack>
      <Box
        sx={{
          mt: 2,
          width: '100%',
          overflowX: 'auto',
        }}
      >
        <Table size="small">
          <TableHead>
            {instance.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableCell
                    key={header.id}
                    colSpan={header.colSpan}
                    sx={{
                      width: header.getSize(),
                      fontWeight: 'bold',
                    }}
                  >
                    {header.column.getCanSort() ? (
                      <Box
                        sx={{
                          cursor: 'pointer',
                          userSelect: 'none',
                        }}
                        onClick={header.column.getToggleSortingHandler()}
                      >
                        <Stack direction="row" alignItems="center">
                          {flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )}
                          {{
                            asc: (
                              <ArrowUpward
                                sx={{
                                  fontSize: '1rem',
                                  ml: 0.5,
                                }}
                              />
                            ),
                            desc: (
                              <ArrowDownward
                                sx={{
                                  fontSize: '1rem',
                                  ml: 0.5,
                                }}
                              />
                            ),
                          }[header.column.getIsSorted() as string] ?? null}
                        </Stack>
                      </Box>
                    ) : (
                      flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableHead>
          <TableBody>
            {instance.getRowModel().rows.map((row) => (
              <StyledTableRow
                key={row.id}
                sx={{
                  backgroundColor: row.depth > 0 ? 'rgba(0, 0, 0, 0.04)' : '',
                }}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </StyledTableRow>
            ))}
          </TableBody>
        </Table>
      </Box>
    </Box>
  );
}

/**
 * Helper function to get the first tag of a DAG for sorting
 */
export function getFirstTag(data?: Data): string {
  if (!data || data.kind != ItemKind.DAG) {
    return '';
  }
  const tags = data.dag.dag.tags;
  if (!tags || tags.length == 0) {
    return '';
  }
  return tags[0] || '';
}

/**
 * Helper function to get the status of a DAG for sorting
 */
export function getStatus(data: RowItem): components['schemas']['Status'] {
  if (data.kind != ItemKind.DAG) {
    return 0;
  }
  return data.dag.latestRun.status;
}

/**
 * Helper function to get configuration
 */
function getConfig() {
  return {
    tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
  };
}

export default DAGTable;
