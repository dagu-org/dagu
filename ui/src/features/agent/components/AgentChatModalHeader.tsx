import type { ReactElement } from 'react';

import { PanelLeft, Plus, Shield, ShieldOff, Terminal, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';

import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { formatCost } from '../utils/formatCost';

type Props = {
  sessionId: string | null;
  totalCost?: number;
  isSidebarOpen: boolean;
  onToggleSidebar: () => void;
  onClearSession: () => void;
  onClose: () => void;
  dragHandlers?: ReturnType<typeof useResizableDraggable>['dragHandlers'];
  isMobile?: boolean;
};


export function AgentChatModalHeader({
  totalCost,
  isSidebarOpen,
  onToggleSidebar,
  onClearSession,
  onClose,
  dragHandlers,
  isMobile,
}: Props): ReactElement {
  const { preferences, updatePreference } = useUserPreferences();

  return (
    <div
      className={cn(
        'flex items-center justify-between px-3 py-2 border-b border-border bg-secondary dark:bg-surface',
        !isMobile && 'cursor-move'
      )}
      {...(dragHandlers || {})}
    >
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <button
          onClick={onToggleSidebar}
          className="p-0.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground flex-shrink-0"
          title={isSidebarOpen ? 'Hide sessions' : 'Show sessions'}
        >
          <PanelLeft className="h-4 w-4" />
        </button>
        <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        <span className="text-xs font-medium text-foreground truncate">Agent</span>
      </div>
      {totalCost != null && totalCost > 0 && (
        <span className="text-[10px] text-muted-foreground/60 flex-shrink-0 tabular-nums">
          {formatCost(totalCost)}
        </span>
      )}
      <TooltipProvider delayDuration={300}>
        <div className="flex items-center gap-1 flex-shrink-0">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => updatePreference('safeMode', !preferences.safeMode)}
                className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                aria-label={preferences.safeMode ? 'Disable safe mode' : 'Enable safe mode'}
                aria-pressed={preferences.safeMode}
              >
                {preferences.safeMode ? (
                  <Shield className="h-4 w-4" />
                ) : (
                  <ShieldOff className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>
                {preferences.safeMode
                  ? "Safe mode enabled: dangerous commands require approval"
                  : "Safe mode disabled: all commands execute immediately"}
              </p>
            </TooltipContent>
          </Tooltip>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClearSession}
            className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
            title="New session"
          >
            <Plus className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
            title="Close"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      </TooltipProvider>
    </div>
  );
}
