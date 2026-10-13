/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import { cn } from '@/lib/utils';
import {
  getStatusClass,
  isActiveNodeStatus,
  statusLabelText,
} from '@/lib/status-utils';
import { useI18n } from '@/i18n/I18nProvider';
import MatrixText from '@/components/ui/matrix-text';
import React, { useEffect, useState } from 'react';
import { NodeStatus } from '../../../../api/v1/schema';

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
  /** Text to display; an API status label such as `not_started` is shown as readable text. */
  children: React.ReactNode;
  /** Size variant of the chip */
  size?: 'sm' | 'md' | 'lg';
};

/**
 * NodeStatusChip displays a styled badge based on the node status
 */
function NodeStatusChip({ status, children, size = 'md' }: Props) {
  const { ts } = useI18n();
  const statusClass = getStatusClass(status);
  const label =
    typeof children === 'string' ? ts(statusLabelText(children)) : children;

  // Size classes
  const sizeClasses = {
    sm: 'text-xs py-0.5 px-2',
    md: 'text-sm py-1 px-3',
    lg: 'text-base py-1.5 px-4',
  };

  const isRunning = status === NodeStatus.Running;
  const isActive = isActiveNodeStatus(status);

  // Render a minimal badge - animated spinner for active states, matrix text only for running
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full font-medium',
        statusClass,
        sizeClasses[size]
      )}
    >
      {isActive && (
        <span className="mr-1.5 inline-flex" aria-hidden="true">
          <BrailleSpinner />
        </span>
      )}
      <span className="font-medium break-keep text-nowrap whitespace-nowrap">
        {isRunning && typeof label === 'string' ? (
          <MatrixText text={label} />
        ) : (
          label
        )}
      </span>
    </div>
  );
}

export default NodeStatusChip;
