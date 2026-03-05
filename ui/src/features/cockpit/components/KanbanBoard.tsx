import React, { useCallback, useState } from 'react';
import { components } from '@/api/v1/schema';
import DAGRunDetailsModal from '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal';
import { KanbanColumn } from './KanbanColumn';
import type { KanbanColumns } from '../hooks/useKanbanData';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

interface Props {
  columns: KanbanColumns;
}

export function KanbanBoard({ columns }: Props): React.ReactElement {
  const [selectedRun, setSelectedRun] = useState<DAGRunSummary | null>(null);

  const handleCardClick = useCallback((run: DAGRunSummary) => {
    setSelectedRun(run);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSelectedRun(null);
  }, []);

  return (
    <>
      <div className="flex gap-3 flex-1 min-h-0 overflow-x-auto p-1">
        <KanbanColumn title="Queued" runs={columns.queued} onCardClick={handleCardClick} />
        <KanbanColumn title="Running" runs={columns.running} onCardClick={handleCardClick} />
        <KanbanColumn title="Done" runs={columns.done} onCardClick={handleCardClick} />
        <KanbanColumn title="Failed" runs={columns.failed} onCardClick={handleCardClick} />
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
