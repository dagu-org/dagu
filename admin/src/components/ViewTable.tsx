import React, { CSSProperties } from "react";
import {
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@mui/material";
import { Link } from "react-router-dom";
import { View } from "../models/View";
import StyledTableRow from "./StyledTableRow";
import ViewActions from "./ViewActions";

type Props = {
  views: View[];
  refreshFn: () => Promise<void>;
};

function ViewTable({ views, refreshFn }: Props) {
  return (
    <React.Fragment>
      <Paper>
        <Table size="small" sx={tableStyle}>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {views.map((v) => (
              <StyledTableRow>
                <TableCell className="has-text-weight-semibold">
                  <Link to={`/views/${encodeURI(v.Name)}`}>{v.Name}</Link>
                </TableCell>
                <TableCell>{v.Desc}</TableCell>
                <TableCell>
                  <ViewActions name={v.Name} refresh={refreshFn} />
                </TableCell>
              </StyledTableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>
    </React.Fragment>
  );
}
const tableStyle: CSSProperties = {
  tableLayout: "fixed",
  wordWrap: "break-word",
};

export default ViewTable;
