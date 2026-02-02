/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import { cn } from '@/lib/utils';
import { getStatusClass } from '@/lib/status-utils';
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
  const statusClass = getStatusClass(status);

  // Size classes
  const sizeClasses = {
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  const isRunning = status === NodeStatus.Running;

  // Render a minimal badge - animated spinner for running, text-only for others
  return (
    <div
      className={cn(
        'inline-flex items-center font-medium',
        statusClass,
        sizeClasses[size]
      )}
    >
      {isRunning && (
        <span className="mr-1.5 inline-flex" aria-hidden="true">
          <BrailleSpinner />
        </span>
      )}
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

export default NodeStatusChip;
