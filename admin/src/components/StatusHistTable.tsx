import moment from 'moment';
import React, { CSSProperties } from 'react';
import { DagStatus } from '../api/DAG';
import { StatusFile } from '../models';
import StatusHistTableRow from './StatusHistTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from './BorderedBox';

type Props = {
  logs: StatusFile[];
  gridData: DagStatus[];
  onSelect: (idx: number) => void;
  idx: number;
};

function StatusHistTable({ logs, gridData, onSelect, idx }: Props) {
  return (
    <BorderedBox>
      <Table size="small" sx={tableStyle}>
        <TableHead>
          <TableRow>
            <TableCell></TableCell>
            {logs.map((log, i) => {
              let date;
              const startedAt = logs[i].Status.StartedAt;
              if (startedAt && startedAt != '-') {
                date = moment(startedAt).format('M/D');
              } else {
                date = moment().format('M/D');
              }
              const flag =
                i == 0 ||
                moment(logs[i - 1].Status.StartedAt).format('M/D') != date;
              return (
                <TableCell
                  key={log.Status.StartedAt}
                  style={colStyle}
                  onClick={() => {
                    onSelect(i);
                  }}
                >
                  {flag ? date : ''}
                </TableCell>
              );
            })}
          </TableRow>
        </TableHead>
        <TableBody>
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
        </TableBody>
      </Table>
    </BorderedBox>
  );
}

export default StatusHistTable;

const colStyle: CSSProperties = {
  maxWidth: '22px',
  minWidth: '22px',
  textAlign: 'left',
};

const tableStyle: CSSProperties = {
  userSelect: 'none',
};
