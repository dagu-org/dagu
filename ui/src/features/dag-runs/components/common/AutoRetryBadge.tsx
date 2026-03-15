import { cn } from '@/lib/utils';

type AutoRetryBadgeProps = {
  count?: number | null;
  limit?: number | null;
  className?: string;
};

function AutoRetryBadge({
  count = 0,
  limit,
  className,
}: AutoRetryBadgeProps) {
  if (!limit || limit <= 0) {
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
        'inline-flex items-center whitespace-nowrap text-xs leading-none text-muted-foreground',
        className
      )}
    >
      {label}
    </span>
  );
}

export default AutoRetryBadge;
