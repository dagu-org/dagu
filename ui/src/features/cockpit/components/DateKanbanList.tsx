import React, { useCallback, useEffect, useRef, useState } from 'react';
import { components } from '@/api/v1/schema';
import DAGRunDetailsModal from '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal';
import { useInfiniteKanban } from '../hooks/useInfiniteKanban';
import { DateKanbanSection } from './DateKanbanSection';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  selectedWorkspace: string;
}

export function DateKanbanList({ selectedWorkspace }: Props): React.ReactElement {
  const { loadedDates, todayStr, hasMore, loadNextDate } = useInfiniteKanban(selectedWorkspace);
  const [selectedRun, setSelectedRun] = useState<DAGRunSummary | null>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const handleCardClick = useCallback((run: DAGRunSummary) => {
    setSelectedRun(run);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSelectedRun(null);
  }, []);

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !hasMore) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) loadNextDate();
      },
      { threshold: 0.1 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasMore, loadNextDate]);

  return (
    <>
      <div className="flex flex-col overflow-y-auto flex-1 min-h-0 gap-6 p-1">
        {loadedDates.map((date) => (
          <DateKanbanSection
            key={date}
            date={date}
            todayStr={todayStr}
            selectedWorkspace={selectedWorkspace}
            onCardClick={handleCardClick}
          />
        ))}
        {hasMore && (
          <div ref={sentinelRef} className="h-1 shrink-0" />
        )}
      </div>
      {selectedRun && (
        <DAGRunDetailsModal
          name={selectedRun.name}
          dagRunId={selectedRun.dagRunId}
          isOpen={!!selectedRun}
          onClose={handleCloseModal}
        />
      )}
    </>
  );
}
