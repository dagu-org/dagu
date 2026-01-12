import React, { useState, useContext, useEffect } from 'react';
import { ChevronRight, Copy, Check, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Markdown } from '@/components/ui/markdown';
import { useQuery } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ChatMessageRole } from '@/api/v2/schema';

interface StepMessagesTableProps {
  dagName: string;
  dagRunId: string;
  stepName: string;
  isRunning: boolean;
  // For sub-DAG runs
  subDAGRunId?: string;
  rootDagName?: string;
  rootDagRunId?: string;
}

const roleConfig: Record<
  string,
  { label: string; borderColor: string }
> = {
  [ChatMessageRole.system]: { label: 'SYS', borderColor: 'border-l-amber-500' },
  [ChatMessageRole.user]: { label: 'USER', borderColor: 'border-l-blue-500' },
  [ChatMessageRole.assistant]: { label: 'ASST', borderColor: 'border-l-green-500' },
  [ChatMessageRole.tool]: { label: 'TOOL', borderColor: 'border-l-purple-500' },
};

const defaultRoleConfig = { label: 'MSG', borderColor: 'border-l-gray-500' };

export function StepMessagesTable({
  dagName,
  dagRunId,
  stepName,
  isRunning,
  subDAGRunId,
  rootDagName,
  rootDagRunId,
}: StepMessagesTableProps) {
  const appBarContext = useContext(AppBarContext);
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  // Determine if this is a sub-DAG run
  const isSubDAGRun = !!subDAGRunId;
  const effectiveName = isSubDAGRun ? (rootDagName || dagName) : dagName;
  const effectiveRunId = isSubDAGRun ? (rootDagRunId || dagRunId) : dagRunId;

  // Query for regular DAG runs
  const regularQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/steps/{stepName}/messages',
    {
      params: {
        path: { name: effectiveName, dagRunId: effectiveRunId, stepName },
        query: { remoteNode: appBarContext.selectedRemoteNode || 'local' },
      },
    },
    {
      refreshInterval: isRunning ? 2000 : 0,
      revalidateOnFocus: false,
      isPaused: () => isSubDAGRun,
    }
  );

  // Query for sub-DAG runs
  const subDagQuery = useQuery(
    '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/messages',
    {
      params: {
        path: { name: effectiveName, dagRunId: effectiveRunId, subDAGRunId: subDAGRunId || '', stepName },
        query: { remoteNode: appBarContext.selectedRemoteNode || 'local' },
      },
    },
    {
      refreshInterval: isRunning ? 2000 : 0,
      revalidateOnFocus: false,
      isPaused: () => !isSubDAGRun,
    }
  );

  // Use the appropriate query result
  const { data, isLoading } = isSubDAGRun ? subDagQuery : regularQuery;

  const messages = data?.messages || [];

  // Start with empty set; useEffect will expand the last message when data arrives
  const [expandedIndexes, setExpandedIndexes] = useState<Set<number>>(
    new Set()
  );

  // Update expanded state when messages change (expand new last message)
  useEffect(() => {
    if (messages.length > 0) {
      setExpandedIndexes((prev) => {
        const newSet = new Set(prev);
        newSet.add(messages.length - 1);
        return newSet;
      });
    }
  }, [messages.length]);

  const toggleExpand = (index: number) => {
    setExpandedIndexes((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  };

  const handleCopy = async (content: string, index: number) => {
    try {
      await navigator.clipboard.writeText(content);
      setCopiedIndex(index);
      setTimeout(() => setCopiedIndex(null), 2000);
    } catch (err) {
      console.warn('Clipboard access denied:', err);
    }
  };

  if (isLoading && messages.length === 0) {
    return (
      <div className="text-xs text-muted-foreground p-2 flex items-center gap-2">
        <Loader2 className="h-3 w-3 animate-spin" />
        Loading messages...
      </div>
    );
  }

  if (messages.length === 0) {
    return (
      <div className="text-xs text-muted-foreground p-2">
        No messages recorded
      </div>
    );
  }

  return (
    <div className="border rounded bg-white">
      {messages.map((msg, i) => {
        const isExpanded = expandedIndexes.has(i);
        const config = roleConfig[msg.role] || defaultRoleConfig;

        return (
          <div
            key={i}
            className={cn(
              'border-l-2 border-b last:border-b-0 bg-white',
              config.borderColor
            )}
          >
            {/* Header row - always visible, clickable */}
            <button
              onClick={() => toggleExpand(i)}
              className="w-full flex items-center gap-2 px-2 py-1 hover:bg-accent/50 text-left"
              type="button"
            >
              <ChevronRight
                className={cn(
                  'h-3 w-3 shrink-0 transition-transform',
                  isExpanded && 'rotate-90'
                )}
              />
              <span className="text-xs font-mono w-10 shrink-0 text-muted-foreground">
                {config.label}
              </span>
              {!isExpanded && (
                <>
                  <span className="text-xs text-muted-foreground truncate flex-1 min-w-0">
                    {msg.content.slice(0, 80)}
                    {msg.content.length > 80 && '...'}
                  </span>
                  {msg.metadata && (
                    <span className="text-xs text-muted-foreground font-mono shrink-0">
                      {msg.metadata.model} {msg.metadata.totalTokens}t
                    </span>
                  )}
                </>
              )}
            </button>

            {/* Expanded content */}
            {isExpanded && (
              <div className="px-2 pb-2 pl-7">
                <div className="flex gap-4 items-start">
                  <div className="flex-1 min-w-0">
                    <Markdown content={msg.content} />
                  </div>
                  <div className="shrink-0 flex flex-col items-end gap-1">
                    {msg.metadata && (
                      <div className="text-xs text-muted-foreground font-mono text-right">
                        <div>
                          {msg.metadata.provider}/{msg.metadata.model}
                        </div>
                        <div>
                          in:{msg.metadata.promptTokens} out:{msg.metadata.completionTokens}
                        </div>
                      </div>
                    )}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleCopy(msg.content, i);
                      }}
                      className="p-1 hover:bg-accent rounded"
                      type="button"
                      title="Copy message"
                    >
                      {copiedIndex === i ? (
                        <Check className="h-3 w-3 text-green-500" />
                      ) : (
                        <Copy className="h-3 w-3 text-muted-foreground" />
                      )}
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
