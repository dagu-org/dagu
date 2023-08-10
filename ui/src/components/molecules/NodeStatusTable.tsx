import React, { CSSProperties } from 'react';
import { stepTabColStyles } from '../../consts';
import { useDAGPostAPI } from '../../hooks/useDAGPostAPI';
import { Node } from '../../models';
import { SchedulerStatus, Status } from '../../models';
import { Step } from '../../models';
import NodeStatusTableRow from './NodeStatusTableRow';
import StatusUpdateModal from './StatusUpdateModal';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../atoms/BorderedBox';

type Props = {
  nodes?: Node[];
  file?: string;
  status: Status;
  name: string;
  refresh: () => void;
};

function NodeStatusTable({ nodes, status, name, refresh, file = '' }: Props) {
  const [modal, setModal] = React.useState(false);
  const [current, setCurrent] = React.useState<Step | undefined>(undefined);
  const { doPost } = useDAGPostAPI({
    name,
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
      refresh();
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
      <BorderedBox>
        <Table size="small" sx={tableStyle}>
          <TableHead>
            <TableRow>
              <TableCell style={styles[i++]}>No</TableCell>
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
                name={name}
                onRequireModal={requireModal}
              ></NodeStatusTableRow>
            ))}
          </TableBody>
        </Table>
      </BorderedBox>
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
  tableLayout: 'fixed',
  wordWrap: 'break-word',
};
