import moment from 'moment';
import React, { CSSProperties } from 'react';
import HistoryTableRow from './HistoryTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../atoms/BorderedBox';
import { components } from '../../api/v2/schema';

type Props = {
  runs: components['schemas']['RunDetails'][];
  gridData: components['schemas']['DAGLogGridItem'][];
  onSelect: (idx: number) => void;
  idx: number;
};

function HistoryTable({ runs, gridData, onSelect, idx }: Props) {
  return (
    <BorderedBox>
      <Table size="small" sx={tableStyle}>
        <TableHead>
          <TableRow>
            <TableCell></TableCell>
            {runs.map((_, i) => {
              if (!runs || i >= runs.length || !runs[i]) {
                return null;
              }
              let date;
              const startedAt = runs[i].startedAt;
              if (startedAt && startedAt != '-') {
                date = moment(startedAt).format('M/D');
              } else {
                date = moment().format('M/D');
              }
              let flag = false;
              if (i == 0) {
                flag = true;
              }
              if (i > 0 && runs[i - 1]) {
                flag = moment(runs[i - 1]!.startedAt).format('M/D') != date;
              }
              return (
                <TableCell
                  key={`date-${i}`}
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
                key={data.name}
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
