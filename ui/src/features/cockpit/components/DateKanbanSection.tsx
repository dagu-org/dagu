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
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [shouldLoad, setShouldLoad] = React.useState(isToday);

  React.useEffect(() => {
    if (shouldLoad) {
      return;
    }
    const el = containerRef.current;
    if (!el) {
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setShouldLoad(true);
        }
      },
      { rootMargin: '400px 0px' }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [shouldLoad]);

  const { columns, error, isLoading, isEmpty } = useDateKanbanData(
    date,
    selectedWorkspace,
    isToday,
    shouldLoad
  );

  return (
    <div ref={containerRef}>
      <div className="px-1 pb-2">
        <h2 className="text-sm font-semibold text-foreground">
          {formatDateHeader(date)}
        </h2>
      </div>
      {!shouldLoad ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">Scroll to load runs...</div>
      ) : isLoading ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">Loading runs...</div>
      ) : error ? (
        <div className="px-1 py-3 text-xs text-destructive">
          {(error as { message?: string } | undefined)?.message ||
            'Failed to load runs'}
        </div>
      ) : isEmpty ? (
        <div className="px-1 py-3 text-xs text-muted-foreground">No runs</div>
      ) : (
        <KanbanBoard columns={columns} onCardClick={onCardClick} />
      )}
    </div>
  );
}
