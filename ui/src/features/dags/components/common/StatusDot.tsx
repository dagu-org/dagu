import { Status } from '@/api/v2/schema';
import React from 'react';

type Props = {
  status: Status;
  statusLabel?: string;
};

type StatusStyle = {
  bgColor: string;
  animation: string;
};

function getStatusStyle(status: Status): StatusStyle {
  switch (status) {
    case Status.Success:
      return { bgColor: 'bg-success', animation: '' };
    case Status.Failed:
    case Status.Rejected:
      return { bgColor: 'bg-destructive', animation: '' };
    case Status.Running:
      return { bgColor: 'bg-primary', animation: 'animate-pulse' };
    case Status.Queued:
    case Status.NotStarted:
      return { bgColor: 'bg-info', animation: '' };
    case Status.PartialSuccess:
    case Status.Aborted:
      return { bgColor: 'bg-warning', animation: '' };
    case Status.Waiting:
      return { bgColor: 'bg-warning', animation: 'animate-pulse' };
    default:
      return { bgColor: 'bg-muted', animation: '' };
  }
}

export function StatusDot({ status, statusLabel }: Props): React.JSX.Element {
  const { bgColor, animation } = getStatusStyle(status);

  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${bgColor} ${animation}`}
      title={statusLabel || ''}
    />
  );
}
