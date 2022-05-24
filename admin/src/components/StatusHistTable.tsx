import moment from "moment";
import React, { CSSProperties } from "react";
import { DagStatus } from "../api/Workflow";
import { StatusFile } from "../models/StatusFile";
import StatusHistTableRow from "./StatusHistTableRow";

type Props = {
  logs: StatusFile[];
  gridData: DagStatus[];
  onSelect: (idx: number) => void;
  idx: number;
};

function StatusHistTable({ logs, gridData, onSelect, idx }: Props) {
  return (
    <table className="table is-fullwidth card" style={tableStyle}>
      <thead className="has-background-light">
        <tr>
          <th>Date</th>
          {logs.map((log, i) => {
            let td;
            let startedAt = logs[i].Status.StartedAt;
            if (startedAt && startedAt != "-") {
              td = moment(startedAt).format("M/D");
            } else {
              td = moment().format("M/D");
            }
            const flag =
              i == 0 ||
              moment(logs[i - 1].Status.StartedAt).format("M/D") != td;
            const style: CSSProperties = { ...colstyle };
            if (!flag) {
              style.borderLeft = "none";
            }
            if (i < logs.length - 1) {
              style.borderRight = "none";
            }
            return (
              <th
                key={log.Status.StartedAt}
                style={style}
                onClick={() => {
                  onSelect(i);
                }}
              >
                {flag ? td : ""}
              </th>
            );
          })}
        </tr>
      </thead>
      <tbody>
        {gridData.map((data) => {
          return (
            <StatusHistTableRow
              key={data.Name}
              data={data}
              onSelect={onSelect}
              idx={idx}
            ></StatusHistTableRow>
          );
        })}
      </tbody>
    </table>
  );
}

export default StatusHistTable;

const colstyle = {
  minWidth: "30px",
  maxWidth: "30px",
  width: "30px",
};

const tableStyle: CSSProperties = { userSelect: "none" };
