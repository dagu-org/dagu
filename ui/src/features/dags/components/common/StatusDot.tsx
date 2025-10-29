import { Status } from '@/api/v2/schema';

type Props = {
  status: Status;
  statusLabel?: string;
};

export function StatusDot({ status, statusLabel }: Props) {
  // Match colors from StatusChip.tsx
  let bgColor = '';
  let animation = '';

  switch (status) {
    case Status.Success:
      bgColor = 'bg-[green] dark:bg-[lightgreen]';
      break;
    case Status.Failed:
      bgColor = 'bg-[red] dark:bg-[lightcoral]';
      break;
    case Status.Running:
      bgColor = 'bg-[limegreen] dark:bg-[lime]';
      animation = 'animate-pulse';
      break;
    case Status.Cancelled:
      bgColor = 'bg-[deeppink] dark:bg-[pink]';
      break;
    case Status.NotStarted:
      bgColor = 'bg-[steelblue] dark:bg-[lightblue]';
      break;
    case Status.Queued:
      bgColor = 'bg-[purple] dark:bg-[plum]';
      break;
    case Status.PartialSuccess:
      bgColor = 'bg-[#ea580c] dark:bg-[#f59e0b]';
      break;
    default:
      bgColor = 'bg-[gray] dark:bg-[lightgray]';
  }

  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${bgColor} ${animation}`}
      title={statusLabel || ''}
    />
  );
}
