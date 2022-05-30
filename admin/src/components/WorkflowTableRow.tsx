import { Chip, TableCell } from "@mui/material";
import React from "react";
import { Link } from "react-router-dom";
import { DAG } from "../models/Dag";
import StatusChip from "./StatusChip";
import StyledTableRow from "./StyledTableRow";

type Props = {
  workflow: DAG;
  group: string;
};

function WorkflowTableRow({ workflow, group }: Props) {
  const url = encodeURI(
    "/dags/" + workflow.File.replace(/\.[^/.]+$/, "") + "?group=" + group
  );
  return (
    <StyledTableRow>
      <TableCell className="has-text-weight-semibold">
        <Link to={url}>{workflow.File}</Link>
      </TableCell>
      <TableCell>
        <Chip color="primary" size="small" label="Workflow" />
      </TableCell>
      <TableCell>{workflow.Config!.Name}</TableCell>
      <TableCell>{workflow.Config!.Description}</TableCell>
      <TableCell>
        <StatusChip status={workflow.Status!.Status}>
          {workflow.Status!.StatusText}
        </StatusChip>
      </TableCell>
      <TableCell>
        {workflow.Status!.Pid == -1 ? "" : workflow.Status!.Pid}
      </TableCell>
      <TableCell>{workflow.Status!.StartedAt}</TableCell>
      <TableCell>{workflow.Status!.FinishedAt}</TableCell>
    </StyledTableRow>
  );
}

export default WorkflowTableRow;
