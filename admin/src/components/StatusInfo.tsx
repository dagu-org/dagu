import React, { CSSProperties } from "react";
import { Status } from "../models/Status";
import { WorkflowTabType } from "../models/WorkflowTab";
import StatusTag from "./StatusTag";

type Props = {
  status?: Status;
  name: string;
  group: string;
  file?: string;
};

function StatusInfo({ status, name, group, file = "" }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: "fixed",
    wordWrap: "break-word",
  };
  const styles = statusTabColStyles;
  const url = `/dags/${name}?t=${
    WorkflowTabType.ScLog
  }&group=${group}&file=${encodeURI(file)}`;
  let i = 0;
  if (!status) {
    return null;
  }
  return (
    <div className="mt-4">
      <table className="table is-bordered is-fullwidth card" style={tableStyle}>
        <thead className="has-background-light">
          <tr>
            <th style={styles[i++]}>Request ID</th>
            <th style={styles[i++]}>DAG Name</th>
            <th style={styles[i++]}>Started At</th>
            <th style={styles[i++]}>Finished At</th>
            <th style={styles[i++]}>Status</th>
            <th style={styles[i++]}>Params</th>
            <th style={styles[i++]}>Scheduler Log</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td> {status.RequestId || "-"} </td>
            <td className="has-text-weight-semibold"> {status.Name} </td>
            <td> {status.StartedAt} </td>
            <td> {status.FinishedAt} </td>
            <td>
              <StatusTag status={status.Status}>{status.StatusText}</StatusTag>
            </td>
            <td> {status.Params} </td>
            <td>
              <a href={url}> {status.Log} </a>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
export default StatusInfo;

const statusTabColStyles = [
  { width: "240px" },
  { width: "150px" },
  { width: "150px" },
  { width: "150px" },
  { width: "130px" },
  { width: "130px" },
  {},
];
