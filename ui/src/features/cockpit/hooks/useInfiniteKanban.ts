import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import dayjs from '@/lib/dayjs';

const MAX_DAYS_BACK = 30;

function adjustTz(
  d: dayjs.Dayjs,
  tzOffsetInSec: number | undefined
): dayjs.Dayjs {
  return tzOffsetInSec !== undefined ? d.utcOffset(tzOffsetInSec / 60) : d;
}

function toDateStr(d: dayjs.Dayjs): string {
  return d.format('YYYY-MM-DD');
}

function getInitialDates(tzOffsetInSec: number | undefined): string[] {
  const now = adjustTz(dayjs(), tzOffsetInSec);
  return [toDateStr(now), toDateStr(now.subtract(1, 'day'))];
}

export function useInfiniteKanban(selectedWorkspace: string) {
  const { tzOffsetInSec } = useConfig();
  const [loadedDates, setLoadedDates] = useState<string[]>(() =>
    getInitialDates(tzOffsetInSec)
  );
  const prevWorkspaceRef = useRef(selectedWorkspace);

  // Reset when workspace changes
  useEffect(() => {
    if (prevWorkspaceRef.current !== selectedWorkspace) {
      prevWorkspaceRef.current = selectedWorkspace;
      setLoadedDates(getInitialDates(tzOffsetInSec));
    }
  }, [selectedWorkspace, tzOffsetInSec]);

  const todayStr = useMemo(
    () => toDateStr(adjustTz(dayjs(), tzOffsetInSec)),
    [loadedDates, tzOffsetInSec]
  );

  const hasMore = useMemo(() => {
    if (loadedDates.length === 0) return true;
    const oldest = loadedDates[loadedDates.length - 1]!;
    const now = adjustTz(dayjs(), tzOffsetInSec);
    const diffDays = now.diff(dayjs(oldest), 'day');
    return diffDays < MAX_DAYS_BACK;
  }, [loadedDates, tzOffsetInSec]);

  const loadNextDate = useCallback(() => {
    setLoadedDates((prev) => {
      if (prev.length === 0) return prev;
      const oldest = prev[prev.length - 1]!;
      const prevDay = dayjs(oldest).subtract(1, 'day');
      return [...prev, toDateStr(prevDay)];
    });
  }, []);

  return { loadedDates, todayStr, hasMore, loadNextDate };
}
