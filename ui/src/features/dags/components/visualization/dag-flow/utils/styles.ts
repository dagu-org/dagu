import { NodeStatus } from '@/api/v2/schema';

/** Border colors for each node status */
export const statusBorderColors: Record<NodeStatus, { light: string; dark: string }> = {
  [NodeStatus.NotStarted]: { dark: '#2d336d', light: '#94a3b8' },
  [NodeStatus.Running]: { dark: '#10b981', light: '#10b981' },
  [NodeStatus.Failed]: { dark: '#ef4444', light: '#ef4444' },
  [NodeStatus.Aborted]: { dark: '#ec4899', light: '#ec4899' },
  [NodeStatus.Success]: { dark: '#10b981', light: '#10b981' },
  [NodeStatus.Skipped]: { dark: '#64748b', light: '#64748b' },
  [NodeStatus.PartialSuccess]: { dark: '#f59e0b', light: '#f59e0b' },
  [NodeStatus.Waiting]: { dark: '#f59e0b', light: '#f59e0b' },
  [NodeStatus.Rejected]: { dark: '#ef4444', light: '#ef4444' },
};

/** Get border color for a status */
export function getStatusBorderColor(status: NodeStatus, isDarkMode: boolean): string {
  const colors = statusBorderColors[status] || statusBorderColors[NodeStatus.NotStarted];
  return isDarkMode ? colors.dark : colors.light;
}

/** Edge colors based on target node status */
export function getEdgeStyle(targetStatus: NodeStatus): {
  stroke: string;
  strokeWidth: number;
  strokeDasharray?: string;
} {
  switch (targetStatus) {
    case NodeStatus.Failed:
      return {
        stroke: '#c4726a',
        strokeWidth: 1.8,
        strokeDasharray: '3',
      };
    case NodeStatus.Success:
      return {
        stroke: '#7da87d',
        strokeWidth: 1.8,
      };
    default:
      return {
        stroke: '#6b635a',
        strokeWidth: 1,
      };
  }
}

/** CSS classes for node status styling */
export const statusClasses: Record<NodeStatus, string> = {
  [NodeStatus.NotStarted]: 'border-slate-400 dark:border-indigo-900',
  [NodeStatus.Running]: 'border-emerald-500 animate-pulse',
  [NodeStatus.Failed]: 'border-red-500',
  [NodeStatus.Aborted]: 'border-pink-500',
  [NodeStatus.Success]: 'border-emerald-500',
  [NodeStatus.Skipped]: 'border-slate-500',
  [NodeStatus.PartialSuccess]: 'border-amber-500',
  [NodeStatus.Waiting]: 'border-amber-500',
  [NodeStatus.Rejected]: 'border-red-500',
};
