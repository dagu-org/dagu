import { TriggerType } from '@/api/v2/schema';
import { Clock, GitBranch, HelpCircle, RotateCw, User, Webhook } from 'lucide-react';

const triggerTypeConfig: Record<
  TriggerType,
  { icon: typeof Clock; label: string; colorClass: string }
> = {
  scheduler: {
    icon: Clock,
    label: 'Scheduled',
    colorClass: 'text-blue-500 dark:text-blue-400',
  },
  manual: {
    icon: User,
    label: 'Manual',
    colorClass: 'text-green-500 dark:text-green-400',
  },
  webhook: {
    icon: Webhook,
    label: 'Webhook',
    colorClass: 'text-purple-500 dark:text-purple-400',
  },
  subdag: {
    icon: GitBranch,
    label: 'Sub-DAG',
    colorClass: 'text-cyan-500 dark:text-cyan-400',
  },
  retry: {
    icon: RotateCw,
    label: 'Retry',
    colorClass: 'text-orange-500 dark:text-orange-400',
  },
  unknown: {
    icon: HelpCircle,
    label: 'Unknown',
    colorClass: 'text-gray-500 dark:text-gray-400',
  },
};

type Props = {
  type?: TriggerType;
  showLabel?: boolean;
  size?: number;
};

export function TriggerTypeIndicator({
  type,
  showLabel = true,
  size = 14,
}: Props): JSX.Element | null {
  if (!type) {
    return null;
  }

  const config = triggerTypeConfig[type] || triggerTypeConfig.unknown;
  const Icon = config.icon;

  return (
    <div className={`flex items-center gap-1 ${config.colorClass}`}>
      <Icon size={size} />
      {showLabel && <span className="text-xs">{config.label}</span>}
    </div>
  );
}
