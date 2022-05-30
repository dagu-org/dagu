import { Chip, TableCell } from "@mui/material";
import React from "react";
import { Link } from "react-router-dom";
import StyledTableRow from "./StyledTableRow";

type Props = {
  url: string;
  text: string;
};

function WorkflowTableRowGroup({ url, text }: Props) {
  return (
    <StyledTableRow>
      <TableCell className="has-text-weight-semibold">
        <Link to={url}>{text}</Link>
      </TableCell>
      <TableCell>
        <Chip color="secondary" size="small" label="Group" />
      </TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
      <TableCell>-</TableCell>
    </StyledTableRow>
  );
}
export default WorkflowTableRowGroup;
