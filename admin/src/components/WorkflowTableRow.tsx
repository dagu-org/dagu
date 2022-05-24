import React from "react";
import { tagColorMapping } from "../consts";
import { DAG } from "../models/Dag";
import StatusTag from "./StatusTag";

type Props = {
  workflow: DAG;
  group: string;
};

function WorkflowTableRow({ workflow, group }: Props) {
  const url = encodeURI(
    "/dags/" + workflow.File.replace(/\.[^/.]+$/, "") + "?group=" + group
  );
  return (
    <tr>
      <td className="has-text-weight-semibold">
        <a href={url}>{workflow.File}</a>
      </td>
      <td>
        <span
          className="tag has-text-weight-semibold"
          style={tagColorMapping["Workflow"]}
        >
          Workflow
        </span>
      </td>
      <td>{workflow.Config!.Name}</td>
      <td>{workflow.Config!.Description}</td>
      <td>
        <StatusTag status={workflow.Status!.Status}>
          {workflow.Status!.StatusText}
        </StatusTag>
      </td>
      <td>{workflow.Status!.Pid == -1 ? "" : workflow.Status!.Pid}</td>
      <td>{workflow.Status!.StartedAt}</td>
      <td>{workflow.Status!.FinishedAt}</td>
    </tr>
  );
}

export default WorkflowTableRow;
