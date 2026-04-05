// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { ChevronRight } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import QueueRunsTable from './QueueRunsTable';

interface QueueCardProps {
  queue: components['schemas']['Queue'];
  isSelected?: boolean;
  onDAGRunClick: (dagRun: components['schemas']['DAGRunSummary']) => void;
}

function QueueCard({ queue, isSelected, onDAGRunClick }: QueueCardProps) {
  const utilization = React.useMemo(() => {
    if (queue.type !== 'global' || !queue.maxConcurrency) {
      return null;
    }
    const running = queue.runningCount || 0;
    return Math.round((running / queue.maxConcurrency) * 100);
  }, [queue]);

  const queuedPreview = queue.queuedPreview ?? [];
  const previewCount = queuedPreview.length;
  const hasMoreQueuedPreview = (queue.queuedCount || 0) > previewCount;

  return (
    <div
      className={cn(
        'card-obsidian transition-all duration-300 dark:hover:bg-white/[0.05] dark:hover:border-white/10',
        isSelected && 'shadow-[0_0_20px_rgba(var(--primary-rgb),0.1)]'
      )}
    >
      <div className="border-b px-3 py-2">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-sm">{queue.name}</span>
                <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                  {queue.type}
                </span>
              </div>
              {queue.type === 'global' && queue.maxConcurrency && (
                <div className="mt-2 flex items-center gap-2">
                  <div className="h-1 w-20 overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full bg-foreground/40 transition-all duration-300"
                      style={{ width: `${utilization || 0}%` }}
                    />
                  </div>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {queue.runningCount || 0}/{queue.maxConcurrency}
                  </span>
                </div>
              )}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3 lg:justify-end">
            <div className="flex items-baseline gap-1 text-xs text-muted-foreground">
              <span className="text-sm font-light tabular-nums text-foreground">
                {queue.runningCount || 0}
              </span>
              <span>running</span>
            </div>
            <div className="flex items-baseline gap-1 text-xs text-muted-foreground">
              <span
                className={cn(
                  'text-sm font-light tabular-nums',
                  (queue.queuedCount || 0) > 0
                    ? 'text-foreground'
                    : 'text-muted-foreground/50'
                )}
              >
                {queue.queuedCount || 0}
              </span>
              <span>queued</span>
            </div>
            {utilization !== null && (
              <div className="flex items-baseline gap-1 text-xs text-muted-foreground">
                <span className="text-sm font-light tabular-nums text-foreground">
                  {utilization}%
                </span>
                <span>util</span>
              </div>
            )}
            <Button variant="outline" size="sm" asChild>
              <Link to={`/queues/${encodeURIComponent(queue.name)}`}>
                View queue
                <ChevronRight className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </div>
        </div>
      </div>

      <div>
        {queue.running && queue.running.length > 0 && (
          <div className="px-3 py-2 bg-muted/10">
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Running ({queue.running.length})
              </span>
            </div>
            <QueueRunsTable
              items={queue.running}
              onDAGRunClick={onDAGRunClick}
            />
          </div>
        )}

        {queue.queuedCount > 0 && (
          <div className={cn(queue.running.length > 0 && 'border-t')}>
            <div className="px-3 py-2 bg-muted/10">
              <div className="mb-2 flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
                <div className="flex flex-wrap items-center gap-3">
                  <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Queued ({queue.queuedCount})
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {previewCount === 0
                      ? 'Preview unavailable'
                      : hasMoreQueuedPreview
                        ? `Previewing ${previewCount} of ${queue.queuedCount}`
                        : `${previewCount} previewed`}
                  </span>
                </div>
                <Button variant="ghost" size="sm" asChild>
                  <Link to={`/queues/${encodeURIComponent(queue.name)}`}>
                    View queue
                  </Link>
                </Button>
              </div>

              {previewCount > 0 ? (
                <QueueRunsTable
                  items={queuedPreview}
                  onDAGRunClick={onDAGRunClick}
                  showQueuedAt
                />
              ) : (
                <div className="rounded-md border border-dashed px-3 py-4 text-sm text-muted-foreground">
                  Open the queue detail page to inspect the queued backlog.
                </div>
              )}
            </div>
          </div>
        )}

        {queue.running.length === 0 && queue.queuedCount === 0 && (
          <div className="px-3 py-4 text-center text-muted-foreground text-xs">
            No DAGs running or queued
          </div>
        )}
      </div>
    </div>
  );
}

export default QueueCard;
