import type { ReactElement } from 'react';

import { Plus, Shield, ShieldOff, Terminal, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';

import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { SessionWithState } from '../types';
import { formatCost } from '../utils/formatCost';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

type Props = {
  sessionId: string | null;
  sessions: SessionWithState[];
  totalCost?: number;
  onSelectSession: (id: string) => void;
  onClearSession: () => void;
  onClose: () => void;
  dragHandlers?: ReturnType<typeof useResizableDraggable>['dragHandlers'];
  isMobile?: boolean;
};


export function AgentChatModalHeader({
  sessionId,
  sessions,
  totalCost,
  onSelectSession,
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
        <Terminal className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        <Select
          value={sessionId || 'new'}
          onValueChange={onSelectSession}
        >
          <SelectTrigger className="h-6 w-auto max-w-[200px] px-2 text-xs bg-transparent border-none shadow-none hover:bg-accent">
            <SelectValue placeholder="New session" />
          </SelectTrigger>
          <SelectContent className="bg-popover border-border">
            <SelectItem value="new" className="text-xs">
              <div className="flex items-center gap-1.5">
                <Plus className="h-3 w-3" />
                New session
              </div>
            </SelectItem>
            {sessions.map((sess) => (
              <SelectItem
                key={sess.session.id}
                value={sess.session.id}
                className="text-xs"
              >
                <div className="flex items-center gap-1.5">
                  {sess.has_pending_prompt ? (
                    <span className="h-2 w-2 rounded-full bg-orange-400 flex-shrink-0" role="img" aria-label="Waiting for input" />
                  ) : sess.working ? (
                    <span className="h-2 w-2 rounded-full bg-green-500 flex-shrink-0" role="img" aria-label="Running" />
                  ) : null}
                  {formatDate(sess.session.created_at)}
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
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
