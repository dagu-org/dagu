/**
 * HistoryTable component displays a table of execution history for a DAG.
 *
 * @module features/dags/components/dag-execution
 */
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
import BorderedBox from '../../../../ui/BorderedBox';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the HistoryTable component
 */
type Props = {
  /** List of DAG runs */
  runs: components['schemas']['RunDetails'][];
  /** Grid data for visualization */
  gridData: components['schemas']['DAGGridItem'][];
  /** Callback for when a run is selected */
  onSelect: (idx: number) => void;
  /** Currently selected index */
  idx: number;
};

/**
 * Style for table cells
 */
const colStyle: CSSProperties = {
  maxWidth: '22px',
  minWidth: '22px',
  textAlign: 'left',
};

/**
 * Style for the table
 */
const tableStyle: CSSProperties = {
  userSelect: 'none',
};

/**
 * HistoryTable displays a grid of execution history for a DAG
 * with dates as column headers and steps as rows
 */
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

              // Format the date for the column header
              let date;
              const startedAt = runs[i].startedAt;
              if (startedAt && startedAt != '-') {
                date = moment(startedAt).format('M/D');
              } else {
                date = moment().format('M/D');
              }

              // Only show the date if it's different from the previous column
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
              />
            );
          })}
        </TableBody>
      </Table>
    </BorderedBox>
  );
}

export default HistoryTable;
