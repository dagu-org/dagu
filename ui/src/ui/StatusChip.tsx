import { cn } from '@/lib/utils';
import React from 'react';
import { Status } from '../api/v2/schema';
import MatrixText from './MatrixText';

type Props = {
  status?: Status;
  children: React.ReactNode; // Allow ReactNode for flexibility, though we expect string
  size?: 'xs' | 'sm' | 'md' | 'lg'; // Size variants
};

function getStatusClass(status?: Status): string {
  switch (status) {
    case Status.Success:
      return 'status-success';
    case Status.Failed:
    case Status.Rejected:
      return 'status-failed';
    case Status.Running:
      return 'status-running';
    case Status.Queued:
    case Status.NotStarted:
      return 'status-info';
    case Status.PartialSuccess:
    case Status.Waiting:
    case Status.Aborted:
      return 'status-warning';
    default:
      return 'status-muted';
  }
}

function StatusChip({
  status,
  children,
  size = 'md',
}: Props): React.JSX.Element {
  const statusClass = getStatusClass(status);
  const isRunning = status === Status.Running;

  // Size classes
  const sizeClasses = {
    xs: 'text-[10px] py-0 px-1.5',
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  // Render a pill-shaped badge with text
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border font-bold uppercase tracking-wider',
        statusClass,
        sizeClasses[size]
      )}
    >
      <span className="font-bold break-keep text-nowrap whitespace-nowrap">
        {isRunning && typeof children === 'string' ? (
          <MatrixText text={children} />
        ) : (
          children
        )}
      </span>
    </div>
  );
}

export default StatusChip;
