import React, { CSSProperties } from "react";
import { stepTabColStyles } from "../consts";
import { useWorkflowPostApi } from "../hooks/useWorkflowPostApi";
import { Node } from "../models/Node";
import { SchedulerStatus, Status } from "../models/Status";
import { Step } from "../models/Step";
import NodeTableRow from "./NodeTableRow";
import StatusUpdateModal from "./StatusUpdateModal";

type Props = {
  nodes?: Node[];
  file?: string;
  status: Status;
  name: string;
  group: string;
  refresh: () => void;
};

function NodeTable({ nodes, status, group, name, refresh, file = "" }: Props) {
  const [modal, setModal] = React.useState(false);
  const [current, setCurrent] = React.useState<Step | undefined>(undefined);
  const { doPost } = useWorkflowPostApi({
    name,
    group,
    onSuccess: refresh,
    requestId: status.RequestId,
  });
  const requireModal = (step: Step) => {
    if (
      status?.Status != SchedulerStatus.Running &&
      status?.Status != SchedulerStatus.None
    ) {
      setCurrent(step);
      setModal(true);
    }
  };
  const dismissModal = React.useCallback(() => {
    setModal(false);
  }, [setModal]);
  const onUpdateStatus = React.useCallback(
    async (step: Step, action: string) => {
      doPost(action, step.Name);
      dismissModal();
    },
    [refresh, dismissModal]
  );
  const styles = stepTabColStyles;
  let i = 0;
  if (!nodes || !nodes.length) {
    return null;
  }
  return (
    <div className="card mt-4" style={divStyle}>
      <table className="table is-bordered is-fullwidth card" style={tableStyle}>
        <thead className="has-background-light">
          <tr>
            <th style={styles[i++]}>#</th>
            <th style={styles[i++]}>Step Name</th>
            <th style={styles[i++]}>Description</th>
            <th style={styles[i++]}>Command</th>
            <th style={styles[i++]}>Args</th>
            <th style={styles[i++]}>Started At</th>
            <th style={styles[i++]}>Finished At</th>
            <th style={styles[i++]}>Status</th>
            <th style={styles[i++]}>Error</th>
            <th style={styles[i++]}>Log</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((n, idx) => (
            <NodeTableRow
              key={n.Step.Name}
              rownum={idx + 1}
              node={n}
              file={file}
              group={group}
              name={name}
              onRequireModal={requireModal}
            ></NodeTableRow>
          ))}
        </tbody>
      </table>
      <StatusUpdateModal
        visible={modal}
        step={current}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </div>
  );
}

export default NodeTable;

const tableStyle: CSSProperties = {
  tableLayout: "fixed",
  wordWrap: "break-word",
};
const divStyle: CSSProperties = {
  overflowX: "auto",
};
