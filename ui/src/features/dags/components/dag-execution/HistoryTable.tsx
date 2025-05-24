/**
 * HistoryTable component displays a table of execution history for a DAG.
 *
 * @module features/dags/components/dag-execution
 */
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { components } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import HistoryTableRow from './HistoryTableRow';

/**
 * Props for the HistoryTable component
 */
type Props = {
  /** List of DAG workflows */
  workflows: components['schemas']['WorkflowDetails'][];
  /** Grid data for visualization */
  gridData: components['schemas']['DAGGridItem'][];
  /** Callback for when a workflow is selected */
  onSelect: (idx: number) => void;
  /** Currently selected index */
  idx: number;
};

/**
 * HistoryTable displays a grid of execution history for a DAG
 * with dates as column headers and steps as rows
 */
function HistoryTable({ workflows, gridData, onSelect, idx }: Props) {
  return (
    <div className="rounded-xl bg-white overflow-hidden border dark:bg-slate-900 dark:border-slate-800">
      <Table className="select-none border-collapse">
        <TableHeader className="bg-slate-50 dark:bg-slate-800">
          <TableRow className="border-b border-slate-200 dark:border-slate-700">
            <TableHead className="py-3"></TableHead>
            {workflows.map((_, i) => {
              if (!workflows || i >= workflows.length || !workflows[i]) {
                return null;
              }

              // Format the date for the column header
              let date;
              const startedAt = workflows[i].startedAt;
              if (startedAt && startedAt != '-') {
                date = dayjs(startedAt).format('M/D');
              } else {
                date = dayjs().format('M/D');
              }

              // Only show the date if it's different from the previous column
              let flag = false;
              if (i == 0) {
                flag = true;
              }
              if (i > 0 && workflows[i - 1]) {
                flag = dayjs(workflows[i - 1]!.startedAt).format('M/D') != date;
              }

              return (
                <TableHead
                  key={`date-${i}`}
                  className={cn(
                    'max-w-[22px] min-w-[22px] text-left p-2 cursor-pointer text-xs font-medium',
                    'hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors duration-200',
                    i === idx && 'bg-slate-100 dark:bg-slate-700'
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
    </div>
  );
}

export default HistoryTable;
