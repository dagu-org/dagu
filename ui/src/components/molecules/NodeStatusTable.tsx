import React, { CSSProperties } from 'react';
import { stepTabColStyles } from '../../consts';
import { useDAGPostAPI } from '../../hooks/useDAGPostAPI';
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
import { components, Status } from '../../api/v2/schema';

type Props = {
  nodes?: components['schemas']['Node'][];
  status: components['schemas']['RunDetails'];
  name: string;
  refresh: () => void;
};

function NodeStatusTable({ nodes, status, name, refresh }: Props) {
  const [modal, setModal] = React.useState(false);
  const [current, setCurrent] = React.useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  const { doPost } = useDAGPostAPI({
    name,
    onSuccess: refresh,
    requestId: status.requestId,
  });
  const requireModal = (step: components['schemas']['Step']) => {
    if (
      status?.status != Status.Running &&
      status?.status != Status.NotStarted
    ) {
      setCurrent(step);
      setModal(true);
    }
  };
  const dismissModal = () => setModal(false);
  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    action: string
  ) => {
    doPost(action, step.name);
    dismissModal();
    refresh();
  };
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
                key={n.step.name}
                rownum={idx + 1}
                node={n}
                requestId={status.requestId}
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
