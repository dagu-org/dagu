import React from 'react';
import { components } from '@/api/v1/schema';
import { useIsMobile } from '@/hooks/useIsMobile';
import { KanbanColumn } from './KanbanColumn';
import { MobileKanbanBoard } from './MobileKanbanBoard';
import type {
  KanbanColumnData,
  KanbanColumns,
} from '../hooks/useDateKanbanData';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  columns: KanbanColumns;
  onCardClick: (run: DAGRunSummary) => void;
}

type ColumnEntry = {
  title: string;
  data: KanbanColumnData;
};

export function KanbanBoard({
  columns,
  onCardClick,
}: Props): React.ReactElement {
  const isMobile = useIsMobile();
  const columnEntries: ColumnEntry[] = [
    { title: 'Queued', data: columns.queued },
    { title: 'Running', data: columns.running },
    { title: 'Review', data: columns.review },
    { title: 'Done', data: columns.done },
    { title: 'Failed', data: columns.failed },
  ];

  if (isMobile) {
    return <MobileKanbanBoard columns={columns} onCardClick={onCardClick} />;
  }

  return (
    <div className="flex gap-3 min-h-0 overflow-x-auto p-1 max-h-[50vh]">
      {columnEntries.map((column) => (
        <KanbanColumn
          key={column.title}
          title={column.title}
          column={column.data}
          onCardClick={onCardClick}
        />
      ))}
    </div>
  );
}
