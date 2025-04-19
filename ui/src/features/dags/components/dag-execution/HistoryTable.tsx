/**
 * HistoryTable component displays a table of execution history for a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import moment from 'moment-timezone';
import React from 'react';
import HistoryTableRow from './HistoryTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import BorderedBox from '../../../../ui/BorderedBox';
import { components } from '../../../../api/v2/schema';
import { cn } from '@/lib/utils';

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
 * HistoryTable displays a grid of execution history for a DAG
 * with dates as column headers and steps as rows
 */
function HistoryTable({ runs, gridData, onSelect, idx }: Props) {
  return (
    <BorderedBox className="overflow-hidden">
      <Table className="select-none border-collapse">
        <TableHeader>
          <TableRow>
            <TableHead></TableHead>
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
                <TableHead
                  key={`date-${i}`}
                  className={cn(
                    'max-w-[22px] min-w-[22px] text-left p-2 cursor-pointer text-xs font-medium',
                    'hover:bg-slate-50 transition-colors duration-200'
                  )}
                  onClick={() => {
                    onSelect(i);
                  }}
                >
                  {flag ? date : ''}
                </TableHead>
              );
            })}
          </TableRow>
        </TableHeader>
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
