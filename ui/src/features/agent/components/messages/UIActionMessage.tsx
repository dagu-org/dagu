import type React from 'react';
import { ExternalLink } from 'lucide-react';
import { UIAction } from '../../types';

export function UIActionMessage({ action }: { action?: UIAction }): React.ReactNode {
  if (!action || action.type !== 'navigate') {
    return null;
  }

  return (
    <div className="pl-1">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <ExternalLink className="h-3 w-3 flex-shrink-0" />
        <span>Navigating to {action.path ?? '(unknown path)'}</span>
      </div>
    </div>
  );
}
