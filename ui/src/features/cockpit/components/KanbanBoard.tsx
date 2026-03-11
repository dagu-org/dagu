import React from 'react';
import { components } from '@/api/v1/schema';
import { useIsMobile } from '@/hooks/useIsMobile';
import { KanbanColumn } from './KanbanColumn';
import { MobileKanbanBoard } from './MobileKanbanBoard';
import type { KanbanColumns } from '../hooks/useDateKanbanData';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  columns: KanbanColumns;
  onCardClick: (run: DAGRunSummary) => void;
}

export function KanbanBoard({ columns, onCardClick }: Props): React.ReactElement {
  const isMobile = useIsMobile();

  if (isMobile) {
    return <MobileKanbanBoard columns={columns} onCardClick={onCardClick} />;
  }

  return (
    <div className="flex gap-3 min-h-0 overflow-x-auto p-1 max-h-[50vh]">
      <KanbanColumn title="Queued" runs={columns.queued} onCardClick={onCardClick} />
      <KanbanColumn title="Running" runs={columns.running} onCardClick={onCardClick} />
      <KanbanColumn title="Review" runs={columns.review} onCardClick={onCardClick} />
      <KanbanColumn title="Done" runs={columns.done} onCardClick={onCardClick} />
      <KanbanColumn title="Failed" runs={columns.failed} onCardClick={onCardClick} />
    </div>
  );
}
