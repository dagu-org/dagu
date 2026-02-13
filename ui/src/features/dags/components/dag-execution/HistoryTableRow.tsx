import { TableCell } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { components, Status } from '../../../../api/v1/schema';
import StyledTableRow from '../../../../ui/StyledTableRow';

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
 * Get status styling based on DAG run status
 * Uses Status enum values which map to DAG run statuses
 */
function getStatusStyling(status: number) {
  let bgColorClass = '';
  let borderColorClass = '';
  let pulseAnimation = '';

  switch (status) {
    case Status.Success: // 4 - success -> green
      bgColorClass = 'bg-success';
      borderColorClass = 'border-success';
      break;
    case Status.Failed: // 2 - failed -> red
      bgColorClass = 'bg-destructive';
      borderColorClass = 'border-destructive';
      break;
    case Status.Running: // 1 - running -> green with pulse
      bgColorClass = 'bg-success';
      borderColorClass = 'border-success';
      pulseAnimation = 'animate-pulse';
      break;
    case Status.Aborted: // 3 - aborted -> pink
      bgColorClass = 'bg-pink-500';
      borderColorClass = 'border-pink-600';
      break;
    case Status.NotStarted: // 0 - not started -> light blue
      bgColorClass = 'bg-primary/60';
      borderColorClass = 'border-primary';
      break;
    case Status.Queued: // 5 - queued -> info/purple
      bgColorClass = 'bg-info';
      borderColorClass = 'border-info';
      break;
    case Status.PartialSuccess: // 6 - partial success -> warning/amber
      bgColorClass = 'bg-warning';
      borderColorClass = 'border-warning';
      break;
    default: // Fallback to gray
      bgColorClass = 'bg-muted-foreground';
      borderColorClass = 'border-border';
  }

  return { bgColorClass, borderColorClass, pulseAnimation };
}

/**
 * HistoryTableRow displays a row in the execution history table
 * with colored circles representing the status of each run
 */
function HistoryTableRow({ data, onSelect, idx }: Props) {
  return (
    <StyledTableRow className="hover:bg-muted transition-colors duration-200">
      <TableCell className="font-medium text-sm">{data.name}</TableCell>
      {[...data.history].reverse().map((status, i) => {
        // Determine if this cell should be highlighted
        const isSelected = i === idx;
        const { bgColorClass, borderColorClass, pulseAnimation } =
          getStatusStyling(status);

        return (
          <TableCell
            key={i}
            onClick={() => onSelect(i)}
            className={cn(
              'max-w-[22px] min-w-[22px] p-2 text-center cursor-pointer',
              'hover:bg-muted/50 transition-all duration-200',
              isSelected && 'bg-accent'
            )}
          >
            {status !== 0 && (
              <div
                className={cn(
                  'w-[12px] h-[12px] rounded-full border-[1.5px] transition-all duration-300 mx-auto hover:scale-110',
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
