import { cn } from '@/lib/utils';
import { SyncStatus } from '@/api/v2/schema';

type SyncStatusBadgeProps = {
  status: SyncStatus;
  className?: string;
};

const statusConfig: Record<SyncStatus, { label: string; className: string }> = {
  [SyncStatus.synced]: {
    label: 'Synced',
    className: 'bg-green-500/10 text-green-600 dark:text-green-400',
  },
  [SyncStatus.modified]: {
    label: 'Modified',
    className: 'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400',
  },
  [SyncStatus.untracked]: {
    label: 'Untracked',
    className: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  },
  [SyncStatus.conflict]: {
    label: 'Conflict',
    className: 'bg-red-500/10 text-red-600 dark:text-red-400',
  },
};

export function SyncStatusBadge({ status, className }: SyncStatusBadgeProps) {
  const config = statusConfig[status];

  return (
    <span
      className={cn(
        'inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium',
        config.className,
        className
      )}
    >
      {config.label}
    </span>
  );
}
