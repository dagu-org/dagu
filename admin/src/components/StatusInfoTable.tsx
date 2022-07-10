import React, { CSSProperties } from 'react';
import { Status } from '../models/Status';
import { DetailTabId } from '../models/DAGData';
import StatusChip from './StatusChip';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from './BorderedBox';

type Props = {
  status?: Status;
  name: string;
  file?: string;
};

function StatusInfoTable({ status, name, file = '' }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: 'fixed',
    wordWrap: 'break-word',
  };
  const styles = statusTabColStyles;
  const url = `/dags/${name}?t=${DetailTabId.ScLog}&file=${encodeURI(file)}`;
  let i = 0;
  if (!status) {
    return null;
  }
  return (
    <BorderedBox>
      <Table sx={tableStyle}>
        <TableHead>
          <TableRow>
            <TableCell style={styles[i++]}>Request ID</TableCell>
            <TableCell style={styles[i++]}>DAG Name</TableCell>
            <TableCell style={styles[i++]}>Started At</TableCell>
            <TableCell style={styles[i++]}>Finished At</TableCell>
            <TableCell style={styles[i++]}>Status</TableCell>
            <TableCell style={styles[i++]}>Params</TableCell>
            <TableCell style={styles[i++]}>Scheduler Log</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          <TableRow>
            <TableCell> {status.RequestId || '-'} </TableCell>
            <TableCell> {status.Name} </TableCell>
            <TableCell> {status.StartedAt} </TableCell>
            <TableCell> {status.FinishedAt} </TableCell>
            <TableCell>
              <StatusChip status={status.Status}>
                {status.StatusText}
              </StatusChip>
            </TableCell>
            <TableCell> {status.Params} </TableCell>
            <TableCell>
              <a href={url}> {status.Log} </a>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </BorderedBox>
  );
}
export default StatusInfoTable;

const statusTabColStyles = [
  { width: '240px' },
  { width: '150px' },
  { width: '150px' },
  { width: '150px' },
  { width: '130px' },
  { width: '130px' },
  {},
];
