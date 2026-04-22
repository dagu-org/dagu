import React from 'react';

import { Status } from '@/api/v1/schema';
import { Badge } from '@/components/ui/badge';
import { MatrixText } from '@/components/ui/matrix-text';
import { getStatusClass } from '@/lib/status-utils';
import { cn } from '@/lib/utils';

type StatusChipProps = {
  status?: Status;
  children: React.ReactNode;
  size?: 'xs' | 'sm' | 'md' | 'lg';
};

function StatusChip({
  status,
  children,
  size = 'md',
}: StatusChipProps): React.JSX.Element {
  const statusClass = getStatusClass(status);
  const isRunning = status === Status.Running;
  const sizeClasses = {
    xs: 'h-5 px-1.5 text-[10px]',
    sm: 'h-5 px-2 text-[11px]',
    md: 'h-6 px-2.5 text-xs',
    lg: 'h-7 px-3 text-sm',
  };

  return (
    <Badge
      variant="outline"
      className={cn(
        'normal-case tracking-normal border-current/20 bg-current/[0.06]',
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
    </Badge>
  );
}

export type { StatusChipProps };
export { StatusChip };
export default StatusChip;
