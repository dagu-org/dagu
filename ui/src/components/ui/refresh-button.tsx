import React, { useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Button } from './button';
import { cn } from '../../lib/utils';

interface RefreshButtonProps {
  onRefresh: () => void | Promise<void>;
  className?: string;
  disabled?: boolean;
}

export const RefreshButton: React.FC<RefreshButtonProps> = ({
  onRefresh,
  className,
  disabled = false,
}) => {
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await onRefresh();
    } finally {
      // Brief visual feedback
      setTimeout(() => setIsRefreshing(false), 500);
    }
  };

  return (
    <Button
      size="sm"
      onClick={handleRefresh}
      disabled={disabled || isRefreshing}
      className={cn("p-2", className)}
      title="Refresh"
    >
      <RefreshCw
        size={16}
        className={cn(
          isRefreshing && "animate-spin"
        )}
      />
    </Button>
  );
};