import React, { useEffect } from 'react';
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
import { Link, useSearchParams } from 'react-router-dom';
import {
  getFirstTag,
  getStatus,
  getStatusField,
  DAGItem,
  DAGDataType,
  getNextSchedule,
} from '../../models';
import StyledTableRow from '../atoms/StyledTableRow';
import {
  ArrowDownward,
  ArrowUpward,
  KeyboardArrowDown,
  KeyboardArrowUp,
} from '@mui/icons-material';
import LiveSwitch from './LiveSwitch';
import moment from 'moment';
import 'moment-duration-format';
import Ticker from '../atoms/Ticker';
import VisuallyHidden from '../atoms/VisuallyHidden';

type Props = {
  DAGs: DAGItem[];
  group: string;
  refreshFn: () => void;
};

type DAGRow = DAGItem & { subRows?: DAGItem[] };

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

const columnHelper = createColumnHelper<DAGRow>();

const defaultColumns = [
  columnHelper.accessor('Name', {
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
  columnHelper.accessor('Name', {
    id: 'Name',
    cell: ({ row, getValue }) => {
      const data = row.original!;
      if (data.Type == DAGDataType.Group) {
        return getValue();
      } else {
        const name = data.DAGStatus.File.replace(/.y[a]{0,1}ml$/, '');
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
      if (data.Type == DAGDataType.Group) {
        return true;
      } else if (data.Type == DAGDataType.DAG) {
        const value = data.DAGStatus.DAG.Name;
        return value.toLowerCase().includes(filter.toLowerCase());
      }
      return false;
    },
    sortingFn: (a, b) => {
      const ta = a.original!.Type;
      const tb = b.original!.Type;
      if (ta == tb) {
        const dataA = a.original!.Name.toLowerCase();
        const dataB = b.original!.Name.toLowerCase();
        return dataA.localeCompare(dataB);
      }
      if (ta == DAGDataType.Group) {
        return 1;
      }
      return -1;
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Tags',
    header: 'Tags',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        const tags = data.DAGStatus.DAG.Tags;
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
      if (data.Type != DAGDataType.DAG) {
        return true;
      }
      const tags = data.DAGStatus.DAG.Tags;
      const ret = tags?.some((tag) => tag == filter) || false;
      return ret;
    },
    sortingFn: (a, b) => {
      const valA = getFirstTag(a.original);
      const valB = getFirstTag(b.original);
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Status',
    header: 'Status',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return (
          <StatusChip status={data.DAGStatus.Status?.Status}>
            {data.DAGStatus.Status?.StatusText || ''}
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
  columnHelper.accessor('Type', {
    id: 'Started At',
    header: 'Started At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAGStatus.Status?.StartedAt;
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      const valA = getStatusField('StartedAt', dataA);
      const valB = getStatusField('StartedAt', dataB);
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Finished At',
    header: 'Finished At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAGStatus.Status?.FinishedAt;
      }
      return null;
    },
    sortingFn: (a, b) => {
      const dataA = a.original!;
      const dataB = b.original!;
      const valA = getStatusField('FinishedAt', dataA);
      const valB = getStatusField('FinishedAt', dataB);
      return valA.localeCompare(valB);
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Schedule',
    header: 'Schedule',
    enableSorting: true,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        const schedules = data.DAGStatus.DAG.Schedule;
        if (schedules) {
          return (
            <React.Fragment>
              {schedules.map((s) => (
                <Chip
                  key={s.Expression}
                  sx={{
                    fontWeight: 'semibold',
                    marginRight: 1,
                  }}
                  size="small"
                  label={s.Expression}
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
      if (dataA.Type != DAGDataType.DAG || dataB.Type != DAGDataType.DAG) {
        return dataA!.Type - dataB!.Type;
      }
      return (
        getNextSchedule(dataA.DAGStatus) - getNextSchedule(dataB.DAGStatus)
      );
    },
  }),
  columnHelper.accessor('Type', {
    id: 'NextRun',
    header: 'Next Run',
    enableSorting: true,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        const schedules = data.DAGStatus.DAG.Schedule;
        if (schedules && schedules.length && !data.DAGStatus.Suspended) {
          return (
            <React.Fragment>
              in{' '}
              <Ticker intervalMs={1000}>
                {() => {
                  const ms = moment
                    .unix(getNextSchedule(data.DAGStatus))
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
      if (dataA.Type != DAGDataType.DAG || dataB.Type != DAGDataType.DAG) {
        return dataA!.Type - dataB!.Type;
      }
      return (
        getNextSchedule(dataA.DAGStatus) - getNextSchedule(dataB.DAGStatus)
      );
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Config',
    header: 'Description',
    enableSorting: false,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAGStatus.DAG.Description;
      }
      return null;
    },
  }),
  columnHelper.accessor('Type', {
    id: 'Live',
    header: 'Live',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type != DAGDataType.DAG) {
        return false;
      }
      return (
        <LiveSwitch
          DAG={data.DAGStatus}
          refresh={props.table.options.meta?.refreshFn}
          inputProps={{
            'aria-label': `Toggle ${data.Name}`,
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
      if (data.Type == DAGDataType.Group) {
        return null;
      }
      return (
        <DAGActions
          dag={data.DAGStatus.DAG}
          status={data.DAGStatus.Status}
          name={data.DAGStatus.DAG.Name}
          label={false}
          refresh={props.table.options.meta?.refreshFn}
        />
      );
    },
  }),
];

function DAGTable({ DAGs = [], group = '', refreshFn }: Props) {
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

  const [searchParams, setSearchParams] = useSearchParams();
  useEffect(() => {
    const searchText = searchParams.get('search');
    if (searchText) {
      instance.getColumn('Name')?.setFilterValue(searchText);
    }
    const t = searchParams.get('tag');
    if (t) {
      instance.getColumn('Tags')?.setFilterValue(t);
    }
  }, []);

  const addSearchParam = React.useCallback(
    (key: string, value: string) => {
      const ret: { [key: string]: string } = {};
      searchParams.forEach((v, k) => {
        if (v && k !== key) {
          ret[k] = v;
        }
      });
      if (value) {
        ret[key] = value;
      }
      setSearchParams(ret);
    },
    [searchParams, setSearchParams]
  );

  const selectedTag = React.useMemo(() => {
    return (
      (columnFilters.find((filter) => filter.id == 'Tags')?.value as string) ||
      ''
    );
  }, [columnFilters]);

  const [expanded, setExpanded] = React.useState<ExpandedState>({});

  const data = React.useMemo(() => {
    const groups: {
      [key: string]: DAGRow;
    } = {};
    DAGs.forEach((dag) => {
      if (dag.Type == DAGDataType.DAG) {
        const g = dag.DAGStatus.DAG.Group;
        if (g != '') {
          if (!groups[g]) {
            groups[g] = {
              Type: DAGDataType.Group,
              Name: g,
              subRows: [],
            };
          }
          groups[g].subRows!.push(dag);
        }
      }
    });
    const ret: DAGRow[] = [];
    const groupKeys = Object.keys(groups);
    groupKeys.forEach((key) => {
      ret.push(groups[key]);
    });
    return [
      ...ret,
      ...DAGs.filter(
        (dag) =>
          dag.Type == DAGDataType.DAG &&
          dag.DAGStatus.DAG.Group == '' &&
          dag.DAGStatus.DAG.Group == group
      ),
    ];
  }, [DAGs, group]);

  const tagOptions = React.useMemo(() => {
    const map: { [key: string]: boolean } = { '': true };
    DAGs.forEach((data) => {
      if (data.Type == DAGDataType.DAG) {
        data.DAGStatus.DAG.Tags?.forEach((tag) => {
          map[tag] = true;
        });
      }
    });
    const ret = Object.keys(map).sort();
    return ret;
  }, []);

  const instance = useReactTable({
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
          InputProps={{
            value: instance.getColumn('Name')?.getFilterValue(),
            onChange: (e) => {
              const value = e.target.value || '';
              addSearchParam('search', value);
              instance.getColumn('Name')?.setFilterValue(value);
            },
            type: 'search',
          }}
        />
        <Autocomplete<string>
          size="small"
          limitTags={1}
          value={selectedTag}
          options={tagOptions}
          onChange={(_, value) => {
            const v = value || '';
            addSearchParam('tag', v);
            instance.getColumn('Tags')?.setFilterValue(v);
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
                        row.original!.Type == DAGDataType.Group
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
