import { Status } from '@/api/v2/schema';
import { getStatusColors } from '@/lib/status-utils';
import React from 'react';

type Props = {
  status: Status;
  statusLabel?: string;
};

export function StatusDot({ status, statusLabel }: Props): React.JSX.Element {
  const { bgClass, animation } = getStatusColors(status);

  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${bgClass} ${animation}`}
      title={statusLabel || ''}
    />
  );
}
