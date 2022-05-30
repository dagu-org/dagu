import React, { CSSProperties } from "react";
import { stepTabColStyles } from "../consts";
import { useWorkflowPostApi } from "../hooks/useWorkflowPostApi";
import { Node } from "../models/Node";
import { SchedulerStatus, Status } from "../models/Status";
import { Step } from "../models/Step";
import NodeStatusTableRow from "./NodeStatusTableRow";
import StatusUpdateModal from "./StatusUpdateModal";
import {
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@mui/material";

type Props = {
  nodes?: Node[];
  file?: string;
  status: Status;
  name: string;
  group: string;
  refresh: () => void;
};

function NodeStatusTable({
  nodes,
  status,
  group,
  name,
  refresh,
  file = "",
}: Props) {
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
    <React.Fragment>
      <Paper>
        <Table size="small" sx={tableStyle}>
          <TableHead>
            <TableRow>
              <TableCell style={styles[i++]}>#</TableCell>
              <TableCell style={styles[i++]}>Step Name</TableCell>
              <TableCell style={styles[i++]}>Description</TableCell>
              <TableCell style={styles[i++]}>Command</TableCell>
              <TableCell style={styles[i++]}>Args</TableCell>
              <TableCell style={styles[i++]}>Started At</TableCell>
              <TableCell style={styles[i++]}>Finished At</TableCell>
              <TableCell style={styles[i++]}>Status</TableCell>
              <TableCell style={styles[i++]}>Error</TableCell>
              <TableCell style={styles[i++]}>Log</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {nodes.map((n, idx) => (
              <NodeStatusTableRow
                key={n.Step.Name}
                rownum={idx + 1}
                node={n}
                file={file}
                group={group}
                name={name}
                onRequireModal={requireModal}
              ></NodeStatusTableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>
      <StatusUpdateModal
        visible={modal}
        step={current}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />
    </React.Fragment>
  );
}

export default NodeStatusTable;

const tableStyle: CSSProperties = {
  tableLayout: "fixed",
  wordWrap: "break-word",
};
const divStyle: CSSProperties = {
  overflowX: "auto",
};
