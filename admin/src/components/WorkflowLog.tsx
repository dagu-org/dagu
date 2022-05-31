import React from "react";
import { LogFile } from "../api/Workflow";

type Props = {
  log?: LogFile;
};

function WorkflowLog({ log }: Props) {
  if (!log) {
    return <div>No Log</div>;
  }
  return (
    <div>
      <pre>{log.Content}</pre>
    </div>
  );
}

export default WorkflowLog;
