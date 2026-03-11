import React, { useState, useCallback } from 'react';
import { components } from '@/api/v1/schema';
import { Tabs, Tab } from '@/components/ui/tabs';
import { KanbanColumn } from './KanbanColumn';
import type { KanbanColumns } from '../hooks/useDateKanbanData';

type DAGRunSummary = components['schemas']['DAGRunSummary'];

const STORAGE_KEY = 'dagu_cockpit_active_tab';

const COLUMN_KEYS = ['queued', 'running', 'review', 'done', 'failed'] as const;
type ColumnKey = (typeof COLUMN_KEYS)[number];

const COLUMN_LABELS: Record<ColumnKey, string> = {
  queued: 'Queued',
  running: 'Running',
  review: 'Review',
  done: 'Done',
  failed: 'Failed',
};

function getInitialTab(): ColumnKey {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored && COLUMN_KEYS.includes(stored as ColumnKey)) {
    return stored as ColumnKey;
  }
  return 'running';
}

interface Props {
  columns: KanbanColumns;
  onCardClick: (run: DAGRunSummary) => void;
}

export function MobileKanbanBoard({ columns, onCardClick }: Props): React.ReactElement {
  const [activeTab, setActiveTab] = useState<ColumnKey>(getInitialTab);

  const handleTabChange = useCallback((key: ColumnKey) => {
    setActiveTab(key);
    localStorage.setItem(STORAGE_KEY, key);
  }, []);

  return (
    <div className="flex flex-col min-h-0">
      <Tabs className="overflow-x-auto border-b-0 mb-1">
        {COLUMN_KEYS.map((key) => {
          const count = columns[key].length;
          return (
            <Tab
              key={key}
              isActive={activeTab === key}
              onClick={() => handleTabChange(key)}
              className="h-8 px-2 text-xs"
            >
              {COLUMN_LABELS[key]}
              {count > 0 && (
                <span className="ml-1 text-muted-foreground/60">{count}</span>
              )}
            </Tab>
          );
        })}
      </Tabs>
      <KanbanColumn
        title={COLUMN_LABELS[activeTab]}
        runs={columns[activeTab]}
        onCardClick={onCardClick}
        hideHeader
      />
    </div>
  );
}
