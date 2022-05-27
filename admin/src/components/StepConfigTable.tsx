import React, { CSSProperties } from "react";
import { Step } from "../models/Step";
import StepConfigTableRow from "./StepConfigTableRow";

type Props = {
  steps: Step[];
};

function StepConfigTable({ steps }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: "fixed",
    wordWrap: "break-word",
  };
  const styles = StepConfigTabColStyles;
  let i = 0;
  if (!steps.length) {
    return null;
  }
  return (
    <div className="mb-4">
      <table className="table is-bordered is-fullwidth card" style={tableStyle}>
        <thead className="has-background-light">
          <tr>
            <th style={styles[i++]}>Name</th>
            <th style={styles[i++]}>Description</th>
            <th style={styles[i++]}>Command</th>
            <th style={styles[i++]}>Args</th>
            <th style={styles[i++]}>Dir</th>
            <th style={styles[i++]}>Repeat</th>
            <th style={styles[i++]}>Preconditions</th>
          </tr>
        </thead>
        <tbody>
          {steps.map((step, idx) => (
            <StepConfigTableRow key={idx} step={step}></StepConfigTableRow>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export default StepConfigTable;

const StepConfigTabColStyles = [
  { width: "200px" },
  { width: "200px" },
  { width: "300px" },
  { width: "220px" },
  { width: "150px" },
  { width: "80px" },
  {},
];
