/**
 * NodeStatusChip component displays a chip with appropriate styling based on node status.
 *
 * @module features/dags/components/common
 */
import React from 'react';
import { NodeStatus } from '../../../../api/v2/schema';
import { Badge } from '@/components/ui/badge'; // Import Shadcn Badge

/**
 * Props for the NodeStatusChip component
 */
type Props = {
  /** Status code of the node */
  status: NodeStatus;
  /** Text to display in the chip */
  children: React.ReactNode; // Allow ReactNode for flexibility
};

/**
 * NodeStatusChip displays a styled badge based on the node status
 */
function NodeStatusChip({ status, children }: Props) {
  let variant: 'default' | 'secondary' | 'destructive' | 'outline' = 'outline';
  let className = 'text-xs font-medium px-2.5 py-0.5'; // Base style

  switch (status) {
    case NodeStatus.Success:
      // Custom green styling
      className +=
        ' bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300 border border-green-200 dark:border-green-700/40';
      break;
    case NodeStatus.Failed:
      variant = 'destructive';
      break;
    case NodeStatus.Running:
      // Custom blue styling
      className +=
        ' bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300 border border-blue-200 dark:border-blue-700/40';
      break;
    case NodeStatus.Cancelled:
    case NodeStatus.Skipped: // Group Skipped with Cancelled style
      variant = 'secondary';
      className += ' dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600'; // Dark mode secondary adjustment
      break;
    case NodeStatus.NotStarted:
      variant = 'outline';
      break;
    default:
      variant = 'secondary'; // Default fallback for unknown status
  }

  // Capitalize first letter if children is a string
  const displayChildren =
    typeof children === 'string'
      ? children.charAt(0).toUpperCase() + children.slice(1)
      : children;

  return (
    <Badge variant={variant} className={className}>
      {displayChildren}
    </Badge>
  );
}

export default NodeStatusChip;
