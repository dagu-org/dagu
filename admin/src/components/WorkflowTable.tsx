import React from "react";
import WorkflowTableRow from "./WorkflowTableRow";
import GroupItem from "./GroupItem";
import GroupItemBack from "./GroupItemBack";
import { DAG } from "../models/Dag";
import { Group } from "../models/Group";

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
    <table className="table is-bordered is-fullwidth card">
      <thead className="has-background-light">
        <tr>
          <th>Workflow</th>
          <th>Type</th>
          <th>Name</th>
          <th>Description</th>
          <th>Status</th>
          <th>Pid</th>
          <th>Started At</th>
          <th>Finished At</th>
        </tr>
      </thead>
      <tbody>
        {group != "" ? <GroupItemBack></GroupItemBack> : null}
        {groups.map((item) => {
          return <GroupItem key={item.Name} group={item}></GroupItem>;
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
      </tbody>
    </table>
  );
}
export default WorkflowTable;
