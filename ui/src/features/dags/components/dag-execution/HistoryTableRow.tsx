/**
 * HistoryTableRow component renders a single row in the execution history table.
 *
 * @module features/dags/components/dag-execution
 */
import React from 'react';
import { TableCell } from '@/components/ui/table';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { components, NodeStatus } from '../../../../api/v2/schema';
import { cn } from '@/lib/utils';

/**
 * Props for the HistoryTableRow component
 */
type Props = {
  /** Grid data for the row */
  data: components['schemas']['DAGGridItem'];
  /** Callback for when a cell is selected */
  onSelect: (idx: number) => void;
  /** Currently selected index */
  idx: number;
};

/**
 * Get status styling based on node status
 */
function getStatusStyling(status: number) {
  let bgColorClass = '';
  let borderColorClass = '';
  let pulseAnimation = '';

  switch (status) {
    case NodeStatus.Success: // done -> green
      bgColorClass = 'bg-green-600';
      borderColorClass = 'border-green-700';
      break;
    case NodeStatus.Failed: // error -> red
      bgColorClass = 'bg-red-600';
      borderColorClass = 'border-red-700';
      break;
    case NodeStatus.Running: // running -> lime
      bgColorClass = 'bg-lime-500';
      borderColorClass = 'border-lime-600';
      pulseAnimation = 'animate-pulse';
      break;
    case NodeStatus.Aborted: // aborted -> pink
      bgColorClass = 'bg-pink-500';
      borderColorClass = 'border-pink-600';
      break;
    case NodeStatus.Skipped: // skipped -> gray
      bgColorClass = 'bg-gray-500';
      borderColorClass = 'border-gray-600';
      break;
    case NodeStatus.NotStarted: // none -> lightblue
      bgColorClass = 'bg-blue-400';
      borderColorClass = 'border-blue-500';
      break;
    case NodeStatus.PartialSuccess: // partial success -> orange/amber
      bgColorClass = 'bg-amber-500';
      borderColorClass = 'border-amber-600';
      break;
    default: // Fallback to gray
      bgColorClass = 'bg-gray-500';
      borderColorClass = 'border-gray-600';
  }

  return { bgColorClass, borderColorClass, pulseAnimation };
}

/**
 * HistoryTableRow displays a row in the execution history table
 * with colored circles representing the status of each run
 */
function HistoryTableRow({ data, onSelect, idx }: Props) {
  return (
    <StyledTableRow className="hover:bg-slate-50 transition-colors duration-200">
      <TableCell className="font-medium text-sm">{data.name}</TableCell>
      {[...data.history].reverse().map((status, i) => {
        // Determine if this cell should be highlighted
        const isSelected = i === idx;
        const { bgColorClass, borderColorClass, pulseAnimation } =
          getStatusStyling(status);

        return (
          <TableCell
            key={i}
            onClick={() => {
              onSelect(i);
            }}
            className={cn(
              'max-w-[22px] min-w-[22px] p-2 text-center cursor-pointer',
              'hover:bg-slate-200 transition-all duration-200',
              isSelected && 'bg-slate-200'
            )}
          >
            {status !== 0 && (
              <div
                className={cn(
                  'w-[12px] h-[12px] rounded-full border-[1.5px] transition-all duration-300 mx-auto',
                  ' hover:scale-110',
                  bgColorClass,
                  borderColorClass,
                  pulseAnimation
                )}
              />
            )}
          </TableCell>
        );
      })}
    </StyledTableRow>
  );
}

export default HistoryTableRow;
