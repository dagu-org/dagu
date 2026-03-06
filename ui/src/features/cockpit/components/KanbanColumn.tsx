import React from 'react';
import { AnimatePresence, LayoutGroup } from 'framer-motion';
import { components } from '@/api/v1/schema';
import { KanbanCard } from './KanbanCard';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  title: string;
  runs: DAGRunSummary[];
  onCardClick: (run: DAGRunSummary) => void;
}

export function KanbanColumn({ title, runs, onCardClick }: Props): React.ReactElement {
  return (
    <div className="flex flex-col min-w-0 flex-1">
      <div className="flex items-center gap-2 px-1 pb-2">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{title}</span>
        <span className="text-[11px] text-muted-foreground/60">{runs.length}</span>
      </div>
      <div className="flex flex-col gap-1.5 overflow-y-auto min-h-0 flex-1 px-0.5">
        <LayoutGroup>
          <AnimatePresence mode="popLayout">
            {runs.map((run) => (
              <KanbanCard
                key={run.dagRunId}
                run={run}
                onClick={() => onCardClick(run)}
              />
            ))}
          </AnimatePresence>
        </LayoutGroup>
      </div>
    </div>
  );
}
