import { TriggerType } from '@/api/v1/schema';
import type { ReactElement } from 'react';

const labels: Record<TriggerType, string> = {
  scheduler: 'Scheduled',
  manual: 'Manual',
  webhook: 'Webhook',
  subdag: 'Sub-DAG',
  retry: 'Retry',
  unknown: 'Unknown',
};

type Props = {
  type?: TriggerType;
};

export function TriggerTypeIndicator({ type }: Props): ReactElement | null {
  if (!type) {
    return null;
  }

  return (
    <span className="font-medium text-foreground/90 text-xs">
      {labels[type] ?? type}
    </span>
  );
}
