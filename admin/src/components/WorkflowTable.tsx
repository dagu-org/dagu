import React from "react";
import {
  createTable,
  useTableInstance,
  getCoreRowModel,
} from "@tanstack/react-table";
import StatusChip from "./StatusChip";
import {
  Chip,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@mui/material";
import { Link } from "react-router-dom";
import { WorkflowData, WorkflowDataType } from "../models/Workflow";
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
  }),
  table.createDataColumn("Type", {
    id: "Config",
    header: "Description",
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
  }),
];

function WorkflowTable({ workflows = [], group = "" }: Props) {
  const [columns] = React.useState<typeof defaultColumns>(() => [
    ...defaultColumns,
  ]);

  const sorted = React.useMemo(() => {
    return workflows.sort((a, b) => a.Name.localeCompare(b.Name));
  }, [workflows]);

  const instance = useTableInstance(table, {
    data: sorted,
    columns,
    getCoreRowModel: getCoreRowModel(),
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
                {header.isPlaceholder ? null : header.renderHeader()}
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
