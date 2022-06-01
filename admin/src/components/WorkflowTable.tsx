import React from "react";
import {
  createTable,
  useTableInstance,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
} from "@tanstack/react-table";
import StatusChip from "./StatusChip";
import {
  Box,
  Chip,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TableSortLabel,
} from "@mui/material";
import { Link } from "react-router-dom";
import {
  getFirstTag,
  getStatus,
  getStatusField,
  WorkflowData,
  WorkflowDataType,
} from "../models/Workflow";
import StyledTableRow from "./StyledTableRow";

type Props = {
  workflows: WorkflowData[];
  group: string;
};

const table = createTable().setRowType<WorkflowData>().setTableMetaType<{
  group: string;
}>();

const defaultColumns = [
  table.createDataColumn("Name", {
    id: "Workflow",
    header: "Workflow",
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == WorkflowDataType.Group) {
        const url = `/dags/?group=${encodeURI(data.Group.Name)}`;
        return <Link to={url}>{props.getValue()}</Link>;
      } else {
        const group = props.instance.options.meta?.group || "";
        const url = `/dags/${encodeURI(
          data.DAG.File.replace(/\.[^/.]+$/, "")
        )}?group=${encodeURI(group)}`;
        return <Link to={url}>{props.getValue()}</Link>;
      }
    },
  }),
  table.createDataColumn("Type", {
    id: "Type",
    header: "Type",
    cell: (props) => {
      const data = props.row.original!;
      if (data.Type == WorkflowDataType.Group) {
        return <Chip color="secondary" size="small" label="Group" />;
      } else {
        return <Chip color="primary" size="small" label="Workflow" />;
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
      if (data.Type == WorkflowDataType.Workflow) {
        const tags = data.DAG.Config.Tags;
        return (
          <Stack direction="row" spacing={1}>
            {tags?.map((tag) => (
              <Chip key={tag} size="small" label={tag} />
            ))}
          </Stack>
        );
      }
      return null;
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
      if (data.Type == WorkflowDataType.Workflow) {
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
      if (data.Type == WorkflowDataType.Workflow) {
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
      if (data.Type == WorkflowDataType.Workflow) {
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
      if (data.Type == WorkflowDataType.Workflow) {
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
];

function WorkflowTable({ workflows = [], group = "" }: Props) {
  const [columns] = React.useState<typeof defaultColumns>(() => [
    ...defaultColumns,
  ]);

  const [sorting, setSorting] = React.useState<SortingState>([]);

  const instance = useTableInstance(table, {
    data: workflows,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: {
      sorting,
    },
    onSortingChange: setSorting,
    meta: {
      group,
    },
  });

  return (
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
          <StyledTableRow key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id}>{cell.renderCell()}</TableCell>
            ))}
          </StyledTableRow>
        ))}
      </TableBody>
    </Table>
  );
}
export default WorkflowTable;
