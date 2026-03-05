import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

const MAX_DAYS_BACK = 30;

function toDateStr(d: Date): string {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function getInitialDates(): string[] {
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);
  return [toDateStr(today), toDateStr(yesterday)];
}

export function useInfiniteKanban(selectedWorkspace: string) {
  const [loadedDates, setLoadedDates] = useState<string[]>(getInitialDates);
  const prevWorkspaceRef = useRef(selectedWorkspace);

  // Reset when workspace changes
  useEffect(() => {
    if (prevWorkspaceRef.current !== selectedWorkspace) {
      prevWorkspaceRef.current = selectedWorkspace;
      setLoadedDates(getInitialDates());
    }
  }, [selectedWorkspace]);

  const todayStr = useMemo(() => toDateStr(new Date()), []);

  const hasMore = useMemo(() => {
    if (loadedDates.length === 0) return true;
    const oldest = loadedDates[loadedDates.length - 1]!;
    const oldestDate = new Date(oldest + 'T00:00:00');
    const today = new Date();
    const diffDays = Math.floor(
      (today.getTime() - oldestDate.getTime()) / (1000 * 60 * 60 * 24)
    );
    return diffDays < MAX_DAYS_BACK;
  }, [loadedDates]);

  const loadNextDate = useCallback(() => {
    setLoadedDates((prev) => {
      if (prev.length === 0) return prev;
      const oldest = prev[prev.length - 1]!;
      const oldestDate = new Date(oldest + 'T00:00:00');
      oldestDate.setDate(oldestDate.getDate() - 1);
      return [...prev, toDateStr(oldestDate)];
    });
  }, []);

  return { loadedDates, todayStr, hasMore, loadNextDate };
}
