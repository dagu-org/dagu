import { cn } from '@/lib/utils';
import { getStatusClass } from '@/lib/status-utils';
import React from 'react';
import { Status } from '../api/v1/schema';
import MatrixText from './MatrixText';

type Props = {
  status?: Status;
  children: React.ReactNode; // Allow ReactNode for flexibility, though we expect string
  size?: 'xs' | 'sm' | 'md' | 'lg'; // Size variants
};

function StatusChip({
  status,
  children,
  size = 'md',
}: Props): React.JSX.Element {
  const statusClass = getStatusClass(status);
  const isRunning = status === Status.Running;

  // Size classes
  const sizeClasses = {
    xs: 'text-xs py-0 px-1.5',
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  // Render a minimal badge with text
  return (
    <div
      className={cn(
        'inline-flex items-center font-medium',
        statusClass,
        sizeClasses[size]
      )}
    >
      <span className="font-medium break-keep text-nowrap whitespace-nowrap">
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
