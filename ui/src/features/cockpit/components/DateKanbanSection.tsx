import React from 'react';
import { components } from '@/api/v1/schema';
import dayjs from '@/lib/dayjs';
import { useDateKanbanData } from '../hooks/useDateKanbanData';
import { KanbanBoard } from './KanbanBoard';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  date: string;
  todayStr: string;
  selectedWorkspace: string;
  onCardClick: (run: DAGRunSummary) => void;
}

function formatDateHeader(date: string): string {
  return `${date} ${dayjs(date).format('ddd')}`;
}

export function DateKanbanSection({
  date,
  todayStr,
  selectedWorkspace,
  onCardClick,
}: Props): React.ReactElement {
  const isToday = date === todayStr;
  const { columns, isEmpty } = useDateKanbanData(date, selectedWorkspace, isToday);

  return (
    <div>
      <div className="px-1 pb-2">
        <h2 className="text-sm font-semibold text-foreground">
          {formatDateHeader(date)}
        </h2>
      </div>
      {isEmpty ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">No runs</div>
      ) : (
        <KanbanBoard columns={columns} onCardClick={onCardClick} />
      )}
    </div>
  );
}
