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
import DAGActions from './DAGActions';
import StatusChip from '../atoms/StatusChip';
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
import { getNextSchedule } from '../../models';
import StyledTableRow from '../atoms/StyledTableRow';
import {
  ArrowDownward,
  ArrowUpward,
  KeyboardArrowDown,
  KeyboardArrowUp,
} from '@mui/icons-material';
import LiveSwitch from './LiveSwitch';
import 'moment-duration-format';
import Ticker from '../atoms/Ticker';
import VisuallyHidden from '../atoms/VisuallyHidden';
import moment from 'moment-timezone';
import { components } from '../../api/v2/schema';

type Props = {
  dags: components['schemas']['DAGFile'][];
  group: string;
  refreshFn: () => void;
  searchText: string;
  handleSearchTextChange: (searchText: string) => void;
  searchTag: string;
  handleSearchTagChange: (tag: string) => void;
};

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
        const name = data.dag.dag.name.replace(/.y[a]{0,1}ml$/, '');
        const url = `/dags/${encodeURI(name)}`;
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
      const valA = getStatusField('startedAt', dataA);
      const valB = getStatusField('startedAt', dataB);
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
      const valA = getStatusField('finishedAt', dataA);
      const valB = getStatusField('finishedAt', dataB);
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

      const name = data.dag.dag.name;
      return <div>TODO: {name}</div>;

      // return (
      //   <DAGActions
      //     dag={data.dag.dag}
      //     status={data.dag.latestRun}
      //     name={name}
      //     label={false}
      //     refresh={props.table.options.meta?.refreshFn}
      //   />
      // );
    },
  }),
];

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
                const value = e.target.value || '';
                handleSearchTextChange(value);
              },
              type: 'search',
            },
          }}
        />
        <Autocomplete<string, false, false, true>
          size="small"
          limitTags={1}
          value={searchTag}
          freeSolo
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
            const v = value || '';
            handleSearchTagChange(v);
          }}
          renderInput={(params) => (
            <TextField {...params} variant="filled" label="Search Tag" />
          )}
          sx={{ width: '300px', ml: 2 }}
        />
      </Stack>
      <Box
        sx={{
          border: '1px solid #e0e0e0',
          mt: 2,
        }}
      >
        <Table size="small">
          <TableHead>
            {instance.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableCell
                    key={header.id}
                    style={{
                      padding:
                        header.id == 'Expand' || header.id == 'Name'
                          ? '6px 4px'
                          : '6px 16px',
                    }}
                  >
                    {header.column.getCanSort() ? (
                      <Box
                        {...{
                          sx: {
                            cursor: header.column.getCanSort()
                              ? 'pointer'
                              : 'default',
                          },
                          onClick: header.column.getToggleSortingHandler(),
                          className: 'gray-90',
                        }}
                      >
                        <Stack direction="row" alignItems="center">
                          {header.isPlaceholder
                            ? null
                            : flexRender(
                                header.column.columnDef.header,
                                header.getContext()
                              )}
                          {{
                            asc: (
                              <ArrowUpward
                                sx={{
                                  fontSize: '0.95rem',
                                  ml: 1,
                                }}
                                className="gray-90"
                              />
                            ),
                            desc: (
                              <ArrowDownward
                                sx={{
                                  fontSize: '0.95rem',
                                  ml: 1,
                                }}
                                className="gray-90"
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
                style={{ height: '44px', backgroundColor: 'white' }}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    style={{
                      padding:
                        cell.column.id == 'Expand' || cell.column.id == 'Name'
                          ? '6px 4px'
                          : '6px 16px',
                      backgroundColor:
                        row.original!.kind == ItemKind.Group
                          ? '#d4daed'
                          : undefined,
                    }}
                    width={cell.column.id == 'Expand' ? '44px' : undefined}
                  >
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
export default DAGTable;

export function getFirstTag(data?: Data): string {
  if (!data || data.kind != ItemKind.DAG) {
    return '';
  }
  if (!data.dag.dag.tags || !data.dag.dag.tags.length) {
    return '';
  }
  return data.dag.dag.tags[0] || '';
}

export function getStatus(data: RowItem): components['schemas']['Status'] {
  if (data.kind == ItemKind.DAG) {
    return data.dag.latestRun.status;
  }
  return 0;
}

type KeysMatching<T extends object, V> = {
  [K in keyof T]-?: T[K] extends V ? K : never;
}[keyof T];

export function getStatusField(
  field: KeysMatching<components['schemas']['RunSummary'], string>,
  dag: RowItem
): string {
  if (dag?.kind == ItemKind.DAG) {
    return dag.dag.latestRun[field] || '';
  }
  return '';
}
