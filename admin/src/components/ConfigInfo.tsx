import React, { CSSProperties } from "react";
import { Config } from "../models/Config";
import MultilineText from "./MultilineText";

type Props = {
  config: Config;
};

function ConfigInfo({ config }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: "fixed",
    wordWrap: "break-word",
  };
  const styles = configTabColStyles;
  const preconditions = config.Preconditions.map((c) => (
    <li>
      {c.Condition}
      {" => "}
      {c.Expected}
    </li>
  ));
  let i = 0;
  return (
    <div className="mb-4 mt-4 card">
      <table className="table is-bordered is-fullwidth card" style={tableStyle}>
        <thead className="has-background-light">
          <tr>
            <th style={styles[i++]}>Name</th>
            <th style={styles[i++]}>Description</th>
            <th style={styles[i++]}>MaxActiveRuns</th>
            <th style={styles[i++]}>Params</th>
            <th style={styles[i++]}>Preconditions</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td className="has-text-weight-semibold"> {config.Name} </td>
            <td>
              {" "}
              <MultilineText>{config.Description}</MultilineText>
            </td>
            <td> {config.MaxActiveRuns} </td>
            <td> {config.DefaultParams} </td>
            <td>
              <ul>{preconditions}</ul>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

export default ConfigInfo;

const configTabColStyles = [
  { width: "200px" },
  { width: "200px" },
  { width: "150px" },
  { width: "150px" },
  {},
];
