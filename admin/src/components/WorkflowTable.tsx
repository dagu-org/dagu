import React from "react";
import WorkflowTableRow from "./WorkflowTableRow";
import WorkflowTableRowGroup from "./WorkflowTableRowGroup";
import { DAG } from "../models/Dag";
import { Group } from "../models/Group";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@mui/material";

type Props = {
  workflows: DAG[];
  groups: Group[];
  group: string;
};

function WorkflowTable({ workflows = [], groups = [], group = "" }: Props) {
  const sorted = React.useMemo(() => {
    return workflows.sort((a, b) => {
      if (a.File < b.File) {
        return -1;
      }
      if (a.File > b.File) {
        return 1;
      }
      return 0;
    });
  }, [workflows]);
  return (
    <Table size="small">
      <TableHead>
        <TableRow>
          <TableCell>Workflow</TableCell>
          <TableCell>Type</TableCell>
          <TableCell>Tags</TableCell>
          <TableCell>Name</TableCell>
          <TableCell>Description</TableCell>
          <TableCell>Status</TableCell>
          <TableCell>Pid</TableCell>
          <TableCell>Started At</TableCell>
          <TableCell>Finished At</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {group != "" ? (
          <WorkflowTableRowGroup url={encodeURI("/dags/")} text="../"></WorkflowTableRowGroup>
        ) : null}
        {groups.map((item) => {
          const url = encodeURI("/dags/?group=" + item.Name);
          return <WorkflowTableRowGroup key={item.Name} url={url} text={item.Name} />;
        })}
        {sorted
          .filter((wf) => !wf.Error)
          .map((wf) => {
            return (
              <WorkflowTableRow
                key={wf.File}
                workflow={wf}
                group={group}
              ></WorkflowTableRow>
            );
          })}
      </TableBody>
    </Table>
  );
}
export default WorkflowTable;
