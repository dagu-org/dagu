import React from 'react';
import { components } from '@/api/v1/schema';

export type DAGRunSelectionItem = Pick<
  components['schemas']['DAGRunSummary'],
  'name' | 'dagRunId'
>;

export const getDAGRunSelectionKey = ({
  name,
  dagRunId,
}: DAGRunSelectionItem): string => `${name}\u0000${dagRunId}`;

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

export function useBulkDAGRunSelection(
  dagRuns: components['schemas']['DAGRunSummary'][]
) {
  const [selectedKeys, setSelectedKeys] = React.useState<Set<string>>(
    () => new Set()
  );

  React.useEffect(() => {
    const visibleKeys = new Set(dagRuns.map(getDAGRunSelectionKey));
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

  const toggleSelection = React.useCallback((dagRun: DAGRunSelectionItem) => {
    const key = getDAGRunSelectionKey(dagRun);
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

  const selectAllVisible = React.useCallback(() => {
    const next = new Set(dagRuns.map(getDAGRunSelectionKey));
    setSelectedKeys((previous) =>
      areSetsEqual(previous, next) ? previous : next
    );
  }, [dagRuns]);

  const selectedRuns = React.useMemo(
    () =>
      dagRuns
        .filter((dagRun) => selectedKeys.has(getDAGRunSelectionKey(dagRun)))
        .map((dagRun) => ({
          name: dagRun.name,
          dagRunId: dagRun.dagRunId,
        })),
    [dagRuns, selectedKeys]
  );

  const isSelected = React.useCallback(
    (dagRun: DAGRunSelectionItem) =>
      selectedKeys.has(getDAGRunSelectionKey(dagRun)),
    [selectedKeys]
  );

  return {
    clearSelection,
    isSelected,
    selectAllVisible,
    selectedCount: selectedKeys.size,
    selectedKeys,
    selectedRuns,
    toggleSelection,
  };
}
