// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { components } from '@/api/v1/schema';

export type QueueSelectionItem = Pick<
  components['schemas']['DAGRunSummary'],
  'name' | 'dagRunId'
>;

export const getQueueSelectionKey = ({
  name,
  dagRunId,
}: QueueSelectionItem): string => `${name}\u0000${dagRunId}`;

const areSetsEqual = (a: Set<string>, b: Set<string>): boolean => {
  if (a.size !== b.size) {
    return false;
  }

  for (const value of a) {
    if (!b.has(value)) {
      return false;
    }
  }

  return true;
};

export function useQueueSelection(
  dagRuns: components['schemas']['DAGRunSummary'][]
) {
  const [selectedKeys, setSelectedKeys] = React.useState<Set<string>>(
    () => new Set()
  );

  React.useEffect(() => {
    const visibleKeys = new Set(dagRuns.map(getQueueSelectionKey));
    setSelectedKeys((previous) => {
      if (previous.size === 0) {
        return previous;
      }

      const next = new Set<string>();
      for (const key of previous) {
        if (visibleKeys.has(key)) {
          next.add(key);
        }
      }

      return areSetsEqual(previous, next) ? previous : next;
    });
  }, [dagRuns]);

  const toggleSelection = React.useCallback((dagRun: QueueSelectionItem) => {
    const key = getQueueSelectionKey(dagRun);
    setSelectedKeys((previous) => {
      const next = new Set(previous);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  const clearSelection = React.useCallback(() => {
    setSelectedKeys((previous) =>
      previous.size === 0 ? previous : new Set<string>()
    );
  }, []);

  const replaceSelection = React.useCallback((items: QueueSelectionItem[]) => {
    const next = new Set(items.map(getQueueSelectionKey));
    setSelectedKeys((previous) =>
      areSetsEqual(previous, next) ? previous : next
    );
  }, []);

  const selectAllLoaded = React.useCallback(() => {
    const next = new Set(dagRuns.map(getQueueSelectionKey));
    setSelectedKeys((previous) =>
      areSetsEqual(previous, next) ? previous : next
    );
  }, [dagRuns]);

  const selectedRuns = React.useMemo(
    () =>
      dagRuns
        .filter((dagRun) => selectedKeys.has(getQueueSelectionKey(dagRun)))
        .map((dagRun) => ({
          name: dagRun.name,
          dagRunId: dagRun.dagRunId,
        })),
    [dagRuns, selectedKeys]
  );

  const isSelected = React.useCallback(
    (dagRun: QueueSelectionItem) =>
      selectedKeys.has(getQueueSelectionKey(dagRun)),
    [selectedKeys]
  );

  return {
    clearSelection,
    isSelected,
    replaceSelection,
    selectAllLoaded,
    selectedCount: selectedKeys.size,
    selectedKeys,
    selectedRuns,
    toggleSelection,
  };
}
