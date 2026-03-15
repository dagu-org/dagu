import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { RefreshCw } from 'lucide-react';

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

  return (
    <Badge
      variant={exhausted ? 'warning' : 'info'}
      className={cn('gap-1 px-2 normal-case tracking-normal', className)}
    >
      <RefreshCw className="h-3 w-3" />
      <span>{`Auto retry ${normalizedCount}/${limit}`}</span>
    </Badge>
  );
}

export default AutoRetryBadge;
