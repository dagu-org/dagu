import React from 'react';
import { Status } from '../api/v2/schema';
import { Badge } from '@/components/ui/badge'; // Import Shadcn Badge

type Props = {
  status?: Status;
  children: React.ReactNode; // Allow ReactNode for flexibility, though we expect string
};

function StatusChip({ status, children }: Props) {
  let variant: 'default' | 'secondary' | 'destructive' | 'outline' = 'outline';
  let className = 'text-xs font-medium px-2.5 py-0.5'; // Base style

  switch (status) {
    case Status.Success:
      // Custom green styling
      className +=
        ' bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300 border border-green-200 dark:border-green-700/40';
      // variant remains 'outline' conceptually, but styles are custom
      break;
    case Status.Failed:
      variant = 'destructive';
      break;
    case Status.Running:
      // Custom blue styling
      className +=
        ' bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300 border border-blue-200 dark:border-blue-700/40';
      // variant remains 'outline' conceptually, but styles are custom
      break;
    case Status.Cancelled:
      variant = 'secondary';
      className += ' dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600'; // Dark mode secondary adjustment
      break;
    case Status.NotStarted:
      variant = 'outline';
      break;
    default:
      variant = 'secondary'; // Default fallback for unknown or undefined status
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

export default StatusChip;
