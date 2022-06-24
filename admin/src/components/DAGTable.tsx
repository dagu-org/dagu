import React from "react";
import {
  createTable,
  useTableInstance,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  getFilteredRowModel,
  ColumnFiltersState,
} from "@tanstack/react-table";
import DAGActions from "./DAGActions";
import StatusChip from "./StatusChip";
import {
  Autocomplete,
  Box,
  Chip,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
} from "@mui/material";
import { Link } from "react-router-dom";
import {
  getFirstTag,
  getStatus,
  getStatusField,
  DAGItem,
  DAGDataType,
  DAGGroup,
} from "../models/DAGData";
import StyledTableRow from "./StyledTableRow";

type Props = {
  DAGs: DAGItem[];
  group: string;
  refreshFn: () => Promise<void>;
};

const table = createTable()
  .setRowType<DAGItem>()
  .setFilterMetaType<DAGItem>()
  .setTableMetaType<{
    group: string;
    refreshFn: () => Promise<void>;
  }>();

const UpperGroup = "../";

const defaultColumns = [
  table.createDataColumn("Name", {
    id: "DAG",
    header: "DAG",
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.Group) {
        if (data.Name == UpperGroup) {
          return <Link to={`/dags`}>{props.getValue()}</Link>;
        } else {
          return (
            <Link to={`/dags/?group=${encodeURI(data.Name)}`}>
              {props.getValue()}
            </Link>
          );
        }
      } else {
        const name = data.DAG.File.replace(/.y[a]{0,1}ml$/, "");
        const group = props.instance.options.meta?.group || "";
        const url = `/dags/${encodeURI(name)}`;
        return <Link to={url}>{props.getValue()}</Link>;
      }
    },
  }),
  table.createDataColumn("Type", {
    id: "Type",
    header: "Type",
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
  table.createDataColumn("Type", {
    id: "Tags",
    header: "Tags",
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
      let valA = getFirstTag(a.original);
      let valB = getFirstTag(b.original);
      return valA.localeCompare(valB);
    },
  }),
  table.createDataColumn("Type", {
    id: "Config",
    header: "Description",
    enableSorting: false,
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return data.DAG.Config.Description;
      }
      return null;
    },
  }),
  table.createDataColumn("Type", {
    id: "Status",
    header: "Status",
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == DAGDataType.DAG) {
        return (
          <StatusChip status={data.DAG.Status?.Status}>
            {data.DAG.Status?.StatusText || ""}
          </StatusChip>
        );
      }
      return null;
    },
    sortingFn: (a, b) => {
      let valA = getStatus(a.original);
      let valB = getStatus(b.original);
      return valA < valB ? -1 : 1;
    },
  }),
  table.createDataColumn("Type", {
    id: "Started At",
    header: "Started At",
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
      let valA = getStatusField("StartedAt", dataA);
      let valB = getStatusField("StartedAt", dataB);
      return valA.localeCompare(valB);
    },
  }),
  table.createDataColumn("Type", {
    id: "Finished At",
    header: "Finished At",
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
      let valA = getStatusField("FinishedAt", dataA);
      let valB = getStatusField("FinishedAt", dataB);
      return valA.localeCompare(valB);
    },
  }),
  table.createDisplayColumn({
    id: "Actions",
    header: "Actions",
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

function DAGTable({ DAGs = [], group = "", refreshFn }: Props) {
  const [columns] = React.useState<typeof defaultColumns>(() => [
    ...defaultColumns,
  ]);

  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    []
  );
  const [sorting, setSorting] = React.useState<SortingState>([]);
  const [globalFilter, setGlobalFilter] = React.useState("");

  const selectedTag = React.useMemo(() => {
    return (
      (columnFilters.find((filter) => filter.id == "Tags")?.value as string) ||
      ""
    );
  }, [columnFilters]);

  const data = React.useMemo(() => {
    const ret: DAGItem[] = [];
    const groups: {
      [key: string]: DAGGroup;
    } = {};
    DAGs.forEach((dag) => {
      if (dag.Type == DAGDataType.DAG) {
        if (dag.DAG.Config.Group == group) {
          ret.push(dag);
        } else if (group == "") {
          const group = dag.DAG.Config.Group;
          if (!groups[group]) {
            groups[group] = {
              Type: DAGDataType.Group,
              Name: group,
              DAGs: [],
            };
          }
          groups[group].DAGs.push(dag);
        }
      }
    });
    if (group != "") {
      groups[UpperGroup] = {
        Type: DAGDataType.Group,
        Name: UpperGroup,
        DAGs: [],
      };
    }
    const groupKeys = Object.keys(groups);
    return [...groupKeys.map((k) => groups[k]), ...ret];
  }, [DAGs, group]);

  const tagOptions = React.useMemo(() => {
    const map: { [key: string]: boolean } = {};
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
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onGlobalFilterChange: setGlobalFilter,
    onColumnFiltersChange: setColumnFilters,
    globalFilterFn: (row, _, globalFilter) => {
      const data = row.original;
      if (!data) {
        return false;
      }
      if (data.Type == DAGDataType.Group) {
        return data.Name.toLowerCase().includes(globalFilter);
      }
      const DAG = data.DAG;
      if (DAG.Config.Name.toLowerCase().includes(globalFilter)) {
        return true;
      }
      if (DAG.Config.Description.toLowerCase().includes(globalFilter)) {
        return true;
      }
      const tags = DAG.Config?.Tags;
      if (
        tags &&
        tags.some((tag) => tag.toLowerCase().includes(globalFilter))
      ) {
        return true;
      }
      return false;
    },
    state: {
      sorting,
      globalFilter,
      columnFilters,
    },
    onSortingChange: setSorting,
    meta: {
      group,
      refreshFn,
    },
  });

  return (
    <Box>
      <Stack
        sx={{
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "start",
          alignContent: "flex-center",
        }}
      >
        <TextField
          label="Search Text"
          size="small"
          InputProps={{
            value: globalFilter || "",
            onChange: (value) => {
              const data = value.target.value;
              setGlobalFilter(data);
            },
            type: "search",
          }}
        />
        <Autocomplete<string>
          size="small"
          limitTags={1}
          value={selectedTag}
          options={tagOptions}
          onChange={(_, value) => {
            instance.getColumn("Tags").setFilterValue(value || "");
          }}
          renderInput={(params) => <TextField {...params} label="Filter Tag" />}
          sx={{ width: "300px", ml: 1 }}
        />
      </Stack>
      <Table size="small">
        <TableHead>
          {instance.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableCell key={header.id} colSpan={header.colSpan}>
                  <Box
                    {...{
                      sx: {
                        cursor: header.column.getCanSort()
                          ? "pointer"
                          : "default",
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
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableHead>
        <TableBody>
          {instance.getRowModel().rows.map((row) => (
            <StyledTableRow
              key={row.id}
              style={{
                height: "50px",
              }}
            >
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id}>{cell.renderCell()}</TableCell>
              ))}
            </StyledTableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}
export default DAGTable;
