import React from "react";
import { DAG } from "../models/Dag";

type Props = {
  workflows: DAG[];
  errors: string[];
  hasError: boolean;
};

function WorkflowErrors({ workflows, errors, hasError }: Props) {
  if (!workflows || !hasError) {
    return <div></div>;
  }
  return (
    <div className="notification is-danger mt-0 mb-0">
      <div>Please check the below errors!</div>
      <div className="content">
        <ul>
          {workflows
            .filter((w) => w.Error)
            .map((w) => {
              const url = encodeURI(w.File);
              return (
                <li>
                  <a href={url}>{w.File}</a>: {w.ErrorT}{" "}
                </li>
              );
            })}
          {errors.map((e) => (
            <li>{e}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
export default WorkflowErrors;
