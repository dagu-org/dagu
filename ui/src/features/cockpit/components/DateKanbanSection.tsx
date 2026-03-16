import React from 'react';
import { components } from '@/api/v1/schema';
import dayjs from '@/lib/dayjs';
import type { KanbanColumns } from '../hooks/useDateKanbanData';
import { KanbanBoard } from './KanbanBoard';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  date: string;
  todayStr: string;
  columns: KanbanColumns;
  isLoading: boolean;
  onCardClick: (run: DAGRunSummary) => void;
}

function formatDateHeader(date: string): string {
  return `${date} ${dayjs(date).format('ddd')}`;
}

export function DateKanbanSection({
  date,
  columns,
  isLoading,
  onCardClick,
}: Props): React.ReactElement {
  const isEmpty =
    columns.queued.length === 0 &&
    columns.running.length === 0 &&
    columns.review.length === 0 &&
    columns.done.length === 0 &&
    columns.failed.length === 0;

  return (
    <div>
      <div className="px-1 pb-2">
        <h2 className="text-sm font-semibold text-foreground">
          {formatDateHeader(date)}
        </h2>
      </div>
      {isLoading ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">Loading runs...</div>
      ) : isEmpty ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">No runs</div>
      ) : (
        <KanbanBoard columns={columns} onCardClick={onCardClick} />
      )}
    </div>
  );
}
