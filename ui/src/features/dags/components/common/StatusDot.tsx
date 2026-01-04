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
      bgColor = 'bg-[green]';
      break;
    case Status.Failed:
      bgColor = 'bg-[red]';
      break;
    case Status.Running:
      bgColor = 'bg-success';
      animation = 'animate-pulse';
      break;
    case Status.Aborted:
      bgColor = 'bg-[deeppink]';
      break;
    case Status.NotStarted:
      bgColor = 'bg-[steelblue]';
      break;
    case Status.Queued:
      bgColor = 'bg-[purple]';
      break;
    case Status.PartialSuccess:
      bgColor = 'bg-[#ea580c]';
      break;
    case Status.Waiting:
      bgColor = 'bg-[#f59e0b]';
      animation = 'animate-pulse';
      break;
    case Status.Rejected:
      bgColor = 'bg-[#dc2626]';
      break;
    default:
      bgColor = 'bg-[gray]';
  }

  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${bgColor} ${animation}`}
      title={statusLabel || ''}
    />
  );
}
