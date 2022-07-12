import React from 'react';
import {
  createTable,
  useTableInstance,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  getFilteredRowModel,
  ColumnFiltersState,
  ExpandedState,
  getExpandedRowModel,
} from '@tanstack/react-table';
import DAGActions from './DAGActions';
import StatusChip from './StatusChip';
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
  TableSortLabel,
  TextField,
} from '@mui/material';
import { Link } from 'react-router-dom';
import {
  getFirstTag,
  getStatus,
  getStatusField,
  DAGItem,
  DAGDataType,
  getNextSchedule,
} from '../models/DAGData';
import StyledTableRow from './StyledTableRow';
import { KeyboardArrowDown, KeyboardArrowUp } from '@mui/icons-material';

type Props = {
  DAGs: DAGItem[];
  group: string;
  refreshFn: () => Promise<void>;
};

type DAGRow = DAGItem & { subRows?: DAGItem[] };

const table = createTable()
  .setRowType<DAGRow>()
  .setFilterMetaType<DAGRow>()
  .setTableMetaType<{
    group: string;
    refreshFn: () => Promise<void>;
  }>();

const defaultColumns = [
  table.createDataColumn('Name', {
    id: 'Expand',
    header: ({ instance }) => {
      return (
        <IconButton onClick={instance.getToggleAllRowsExpandedHandler()}>
          {instance.getIsAllRowsExpanded() ? (
            <KeyboardArrowUp />
          ) : (
            <KeyboardArrowDown />
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
  table.createDataColumn('Name', {
    id: 'Name',
    cell: ({ row, getValue }) => {
      const data = row.original!;
      if (data.Type == DAGDataType.Group) {
        return getValue();
      } else {
        const name = data.DAG.File.replace(/.y[a]{0,1}ml$/, '');
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
      let value = '';
      if (data.Type == DAGDataType.Group) {
        value = data.Name;
      } else {
        value = data.DAG.Config.Name;
      }
      const ret = value.toLowerCase().includes(filter.toLowerCase());
      return ret;
    },
  }),
  table.createDataColumn('Type', {
    id: 'Type',
    header: 'Type',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.Group) {
        return <Chip color="secondary" size="small" label="Group" />;
      } else {
        return <Chip color="primary" size="small" label="DAG" />;
      }
    },
    sortingFn: (a, b) => {
      const dataA = a.original;
      const dataB = b.original;
      return dataA!.Type - dataB!.Type;
    },
  }),
  table.createDataColumn('Type', {
    id: 'Tags',
    header: 'Tags',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        const tags = data.DAG.Config.Tags;
        return (
          <Stack direction="row" spacing={1}>
            {tags?.map((tag) => (
              <Chip
                key={tag}
                size="small"
                label={tag}
                onClick={() => props.column.setFilterValue(tag)}
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
        return false;
      }
      const tags = data.DAG.Config.Tags;
      const ret = tags?.some((tag) => tag == filter) || false;
      return ret;
    },
    sortingFn: (a, b) => {
      const valA = getFirstTag(a.original);
      const valB = getFirstTag(b.original);
      return valA.localeCompare(valB);
    },
  }),
  table.createDataColumn('Type', {
    id: 'Config',
    header: 'Description',
    enableSorting: false,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAG.Config.Description;
      }
      return null;
    },
  }),
  table.createDataColumn('Type', {
    id: 'Status',
    header: 'Status',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return (
          <StatusChip status={data.DAG.Status?.Status}>
            {data.DAG.Status?.StatusText || ''}
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
  table.createDataColumn('Type', {
    id: 'Started At',
    header: 'Started At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAG.Status?.StartedAt;
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
  table.createDataColumn('Type', {
    id: 'Finished At',
    header: 'Finished At',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAG.Status?.FinishedAt;
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
  table.createDataColumn('Type', {
    id: 'Schedule',
    header: 'Schedule',
    enableSorting: true,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        const schedules = data.DAG.Config.ScheduleExp;
        if (schedules) {
          return (
            <React.Fragment>
              {schedules.map((s) => (
                <Chip
                  key={s}
                  sx={{
                    fontWeight: 'semibold',
                    marginRight: 1,
                  }}
                  size="small"
                  label={s}
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
      return getNextSchedule(dataA.DAG) - getNextSchedule(dataB.DAG);
    },
  }),
  table.createDisplayColumn({
    id: 'Actions',
    header: 'Actions',
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.Group) {
        return null;
      }
      return (
        <DAGActions
          status={data.DAG.Status}
          name={data.DAG.Config.Name}
          label={false}
          refresh={props.instance.options.meta?.refreshFn}
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
  const [sorting, setSorting] = React.useState<SortingState>([]);

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
        const g = dag.DAG.Config.Group;
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
          dag.DAG.Config.Group == '' &&
          dag.DAG.Config.Group == group
      ),
    ];
  }, [DAGs, group]);

  const tagOptions = React.useMemo(() => {
    const map: { [key: string]: boolean } = { '': true };
    DAGs.forEach((data) => {
      if (data.Type == DAGDataType.DAG) {
        data.DAG.Config.Tags?.forEach((tag) => {
          map[tag] = true;
        });
      }
    });
    const ret = Object.keys(map).sort();
    return ret;
  }, []);

  const instance = useTableInstance(table, {
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
    debugAll: true,
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
            value: instance.getColumn('Name').getFilterValue(),
            onChange: (e) => {
              instance.getColumn('Name').setFilterValue(e.target.value || '');
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
            instance.getColumn('Tags').setFilterValue(value || '');
          }}
          renderInput={(params) => (
            <TextField {...params} variant="filled" label="Search Tag" />
          )}
          sx={{ width: '300px', ml: 2 }}
        />
      </Stack>
      <Box
        sx={{
          border: '1px solid #485fc7',
          borderRadius: '6px',
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
                      <Stack direction="column" spacing={1}>
                        <Box
                          {...{
                            sx: {
                              cursor: header.column.getCanSort()
                                ? 'pointer'
                                : 'default',
                            },
                            onClick: header.column.getToggleSortingHandler(),
                          }}
                        >
                          {header.isPlaceholder ? null : header.renderHeader()}
                          {{
                            asc: <TableSortLabel direction="asc" />,
                            desc: <TableSortLabel direction="desc" />,
                          }[header.column.getIsSorted() as string] ?? null}
                        </Box>
                      </Stack>
                    ) : (
                      header.renderHeader()
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableHead>
          <TableBody>
            {instance.getRowModel().rows.map((row) => (
              <StyledTableRow key={row.id} style={{ height: '44px' }}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    style={{
                      padding:
                        cell.column.id == 'Expand' || cell.column.id == 'Name'
                          ? '6px 4px'
                          : '6px 16px',
                    }}
                    width={cell.column.id == 'Expand' ? '44px' : undefined}
                  >
                    {cell.renderCell()}
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
