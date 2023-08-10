import moment from 'moment';
import React, { CSSProperties } from 'react';
import { GridData } from '../../models/api';
import { StatusFile } from '../../models';
import HistoryTableRow from './HistoryTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../atoms/BorderedBox';

type Props = {
  logs: StatusFile[];
  gridData: GridData[];
  onSelect: (idx: number) => void;
  idx: number;
};

function HistoryTable({ logs, gridData, onSelect, idx }: Props) {
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
              <HistoryTableRow
                key={data.Name}
                data={data}
                onSelect={onSelect}
                idx={idx}
              ></HistoryTableRow>
            );
          })}
        </TableBody>
      </Table>
    </BorderedBox>
  );
}

export default HistoryTable;

const colStyle: CSSProperties = {
  maxWidth: '22px',
  minWidth: '22px',
  textAlign: 'left',
};

const tableStyle: CSSProperties = {
  userSelect: 'none',
};
