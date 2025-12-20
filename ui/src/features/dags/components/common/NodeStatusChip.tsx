/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import { cn } from '@/lib/utils';
import React from 'react';
import { NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the NodeStatusChip component
 */
type Props = {
  /** Status code of the node */
  status: NodeStatus;
  /** Text to display in the chip */
  children: React.ReactNode; // Allow ReactNode for flexibility
  /** Size variant of the chip */
  size?: 'sm' | 'md' | 'lg';
};

/**
 * NodeStatusChip displays a styled badge based on the node status
 */
function NodeStatusChip({ status, children, size = 'md' }: Props) {
  // Determine the colors and icon based on status
  let bgColorClass = '';
  let textColorClass = '';
  let borderColorClass = '';
  let pulseAnimation = '';
  let statusIcon = '';

  switch (status) {
    case NodeStatus.Success: // done -> green
      bgColorClass = 'bg-[rgba(0,128,0,0.1)],100,0,0.2)]';
      borderColorClass = 'border-[green]';
      textColorClass = 'text-[green]';
      statusIcon = '✓'; // Checkmark
      break;
    case NodeStatus.Failed: // error -> red
      bgColorClass = 'bg-[rgba(255,0,0,0.1)],0,0,0.2)]';
      borderColorClass = 'border-[red]';
      textColorClass = 'text-[red]';
      statusIcon = '✕'; // X mark
      break;
    case NodeStatus.Running: // running -> lime
      bgColorClass = 'bg-[rgba(0,255,0,0.1)],205,50,0.2)]';
      borderColorClass = 'border-[lime]';
      textColorClass = 'text-[limegreen]';
      pulseAnimation = 'animate-pulse';
      statusIcon = '●'; // Dot
      break;
    case NodeStatus.Aborted: // aborted -> pink
      bgColorClass =
        'bg-[rgba(255,192,203,0.1)],20,147,0.2)]';
      borderColorClass = 'border-[pink]';
      textColorClass = 'text-[deeppink]';
      statusIcon = '■'; // Square
      break;
    case NodeStatus.Skipped: // skipped -> gray
      bgColorClass =
        'bg-[rgba(128,128,128,0.1)],169,169,0.2)]';
      borderColorClass = 'border-[gray]';
      textColorClass = 'text-[gray]';
      statusIcon = '▫'; // White small square
      break;
    case NodeStatus.NotStarted: // none -> lightblue
      bgColorClass =
        'bg-[rgba(173,216,230,0.1)],130,180,0.2)]';
      borderColorClass = 'border-[lightblue]';
      textColorClass = 'text-[steelblue]';
      statusIcon = '○'; // Circle
      break;
    case NodeStatus.PartialSuccess: // partial success -> orange/amber
      bgColorClass = 'bg-[rgba(245,158,11,0.1)],158,11,0.2)]';
      borderColorClass = 'border-[#f59e0b]';
      textColorClass = 'text-[#f59e0b]';
      statusIcon = '◐'; // Half-filled circle
      break;
    default: // Fallback to gray
      bgColorClass =
        'bg-[rgba(128,128,128,0.1)],169,169,0.2)]';
      borderColorClass = 'border-[gray]';
      textColorClass = 'text-[gray]';
      statusIcon = '○'; // Circle
  }

  // Size classes
  const sizeClasses = {
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  // Render a pill-shaped badge with icon and text
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border',
        bgColorClass,
        borderColorClass,
        textColorClass,
        sizeClasses[size]
      )}
    >
      <span
        className={cn('mr-1.5 inline-flex', pulseAnimation, textColorClass)}
        aria-hidden="true"
      >
        {statusIcon}
      </span>
      <span
        className={cn(
          'font-normal break-keep text-nowrap whitespace-nowrap',
          textColorClass
        )}
      >
        {children}
      </span>
    </div>
  );
}

export default NodeStatusChip;
