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
      bgColor = 'bg-success';
      break;
    case Status.Failed:
    case Status.Rejected:
      bgColor = 'bg-destructive';
      break;
    case Status.Running:
      bgColor = 'bg-primary';
      animation = 'animate-pulse';
      break;
    case Status.Queued:
    case Status.NotStarted:
      bgColor = 'bg-info';
      break;
    case Status.PartialSuccess:
    case Status.Waiting:
    case Status.Aborted:
      bgColor = 'bg-warning';
      if (status === Status.Waiting) animation = 'animate-pulse';
      break;
    default:
      bgColor = 'bg-muted';
  }

  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${bgColor} ${animation}`}
      title={statusLabel || ''}
    />
  );
}
