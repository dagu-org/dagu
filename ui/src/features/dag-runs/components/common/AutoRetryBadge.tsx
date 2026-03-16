// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Status } from '@/api/v1/schema';
import { cn } from '@/lib/utils';

type AutoRetryBadgeProps = {
  status: Status;
  count?: number | null;
  limit?: number | null;
  className?: string;
};

function AutoRetryBadge({
  status,
  count = 0,
  limit,
  className,
}: AutoRetryBadgeProps) {
  if (!limit || limit <= 0) {
    return null;
  }

  if (status !== Status.Failed) {
    return null;
  }

  const normalizedCount = Math.max(count ?? 0, 0);
  const exhausted = normalizedCount >= limit;
  const label = exhausted
    ? 'auto retries exhausted'
    : `${normalizedCount}/${limit} auto retries`;

  return (
    <span
      className={cn(
        'inline-flex items-center whitespace-normal break-words text-xs leading-none text-muted-foreground',
        className
      )}
    >
      {label}
    </span>
  );
}

export default AutoRetryBadge;
