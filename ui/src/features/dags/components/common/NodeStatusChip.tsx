/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import { cn } from '@/lib/utils';
import MatrixText from '@/ui/MatrixText';
import React, { useEffect, useState } from 'react';
import { NodeStatus } from '../../../../api/v2/schema';

const BRAILLE_FRAMES = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'];

function BrailleSpinner() {
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setFrame((prev) => (prev + 1) % BRAILLE_FRAMES.length);
    }, 80);
    return () => clearInterval(interval);
  }, []);

  return <>{BRAILLE_FRAMES[frame]}</>;
}

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
  let statusClass = '';
  let statusIcon = '';

  switch (status) {
    case NodeStatus.Success:
      statusClass = 'status-success';
      statusIcon = '✓';
      break;
    case NodeStatus.Failed:
    case NodeStatus.Rejected:
      statusClass = 'status-failed';
      statusIcon = status === NodeStatus.Failed ? '✕' : '⊘';
      break;
    case NodeStatus.Running:
      statusClass = 'status-running';
      break;
    case NodeStatus.NotStarted:
    case NodeStatus.Skipped:
      statusClass = 'status-info';
      statusIcon = status === NodeStatus.Skipped ? '―' : '○';
      break;
    case NodeStatus.PartialSuccess:
    case NodeStatus.Waiting:
    case NodeStatus.Aborted:
      statusClass = 'status-warning';
      statusIcon =
        status === NodeStatus.Aborted
          ? '■'
          : status === NodeStatus.PartialSuccess
            ? '◐'
            : '□';
      break;
    default:
      statusClass = 'status-muted';
      statusIcon = '○';
  }

  // Size classes
  const sizeClasses = {
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  const isRunning = status === NodeStatus.Running;

  // Render a pill-shaped badge with icon and text
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border font-bold uppercase tracking-wider',
        statusClass,
        sizeClasses[size]
      )}
    >
      <span className="mr-1.5 inline-flex" aria-hidden="true">
        {isRunning ? <BrailleSpinner /> : statusIcon}
      </span>
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

export default NodeStatusChip;
