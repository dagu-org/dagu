import { useCallback, useMemo, useRef, useState } from 'react';
import { DELEGATE_PANEL_BASE_Z_INDEX } from '../constants';
import {
  DelegateEvent,
  DelegateInfo,
  DelegateMessages,
  DelegateSnapshot,
  Message,
} from '../types';

// Consolidated state for a single delegate, combining info, messages, and panel visibility.
interface DelegateState {
  info: DelegateInfo;
  messages: Message[];
  isOpen: boolean;
}

export function useDelegateManager() {
  const [delegateStore, setDelegateStore] = useState<Record<string, DelegateState>>({});
  const delegateStoreRef = useRef<Record<string, DelegateState>>({});
  const zIndexCounterRef = useRef(DELEGATE_PANEL_BASE_Z_INDEX);

  const update = useCallback((next: Record<string, DelegateState>) => {
    delegateStoreRef.current = next;
    return next;
  }, []);

  // Derived views
  const delegates = useMemo(() =>
    Object.values(delegateStore)
      .filter((d) => d.isOpen)
      .map((d, idx) => ({ ...d.info, positionIndex: idx })),
    [delegateStore]
  );
  const delegateStatuses = useMemo(() => {
    const result: Record<string, DelegateInfo> = {};
    for (const [id, entry] of Object.entries(delegateStore)) {
      result[id] = entry.info;
    }
    return result;
  }, [delegateStore]);
  const delegateMessages = useMemo(() => {
    const result: Record<string, Message[]> = {};
    for (const [id, entry] of Object.entries(delegateStore)) {
      result[id] = entry.messages;
    }
    return result;
  }, [delegateStore]);

  // --- SSE event handlers ---

  const handleDelegateSnapshots = useCallback((snapshots: DelegateSnapshot[]) => {
    setDelegateStore((prev) => {
      const next = { ...prev };
      for (const snap of snapshots) {
        next[snap.id] = {
          info: { id: snap.id, task: snap.task, status: snap.status, zIndex: ++zIndexCounterRef.current, positionIndex: 0 },
          messages: prev[snap.id]?.messages || [],
          isOpen: prev[snap.id]?.isOpen || false,
        };
      }
      return update(next);
    });
  }, [update]);

  const handleDelegateMessages = useCallback((dm: DelegateMessages) => {
    const { delegate_id, messages: msgs } = dm;
    setDelegateStore((prev) => {
      const entry = prev[delegate_id] || {
        info: { id: delegate_id, task: '', status: 'running' as const, zIndex: 0, positionIndex: 0 },
        messages: [],
        isOpen: false,
      };
      const existing = entry.messages;
      const idxMap = new Map<string, number>();
      existing.forEach((m, i) => idxMap.set(m.id, i));
      const updated = [...existing];
      for (const msg of msgs) {
        const idx = idxMap.get(msg.id);
        if (idx !== undefined) {
          updated[idx] = msg;
        } else {
          idxMap.set(msg.id, updated.length);
          updated.push(msg);
        }
      }
      return update({ ...prev, [delegate_id]: { ...entry, messages: updated } });
    });
  }, [update]);

  const handleDelegateEvent = useCallback((evt: DelegateEvent) => {
    if (evt.type === 'started') {
      const zIndex = ++zIndexCounterRef.current;
      setDelegateStore((prev) =>
        update({
          ...prev,
          [evt.delegate_id]: {
            info: { id: evt.delegate_id, task: evt.task, status: 'running' as const, zIndex, positionIndex: 0 },
            messages: prev[evt.delegate_id]?.messages || [],
            isOpen: true,
          },
        })
      );
    } else if (evt.type === 'completed') {
      setDelegateStore((prev) => {
        const existing = prev[evt.delegate_id];
        if (!existing) return prev;
        return update({
          ...prev,
          [evt.delegate_id]: { ...existing, info: { ...existing.info, status: 'completed' as const } },
        });
      });
    }
  }, [update]);

  // --- State management ---

  const resetDelegates = useCallback(() => {
    setDelegateStore({});
    delegateStoreRef.current = {};
  }, []);

  const restoreDelegates = useCallback((snapshots: DelegateSnapshot[]) => {
    const newStore: Record<string, DelegateState> = {};
    for (const snap of snapshots) {
      newStore[snap.id] = {
        info: { id: snap.id, task: snap.task, status: snap.status, zIndex: ++zIndexCounterRef.current, positionIndex: 0 },
        messages: [],
        isOpen: false,
      };
    }
    setDelegateStore(newStore);
    delegateStoreRef.current = newStore;
  }, []);

  // --- User actions ---

  const bringToFront = useCallback((delegateId: string) => {
    setDelegateStore((prev) => {
      const entry = prev[delegateId];
      if (!entry) return prev;
      return update({ ...prev, [delegateId]: { ...entry, info: { ...entry.info, zIndex: ++zIndexCounterRef.current } } });
    });
  }, [update]);

  const openDelegate = useCallback((delegateId: string, task: string, messages?: Message[]) => {
    setDelegateStore((prev) => {
      const entry = prev[delegateId];
      if (entry?.isOpen) return prev;
      const info = entry?.info || { id: delegateId, task, status: 'completed' as const, zIndex: 0, positionIndex: 0 };
      return update({
        ...prev,
        [delegateId]: {
          info: { ...info, zIndex: ++zIndexCounterRef.current },
          messages: messages || entry?.messages || [],
          isOpen: true,
        },
      });
    });
  }, [update]);

  const setDelegateMessagesForId = useCallback((delegateId: string, task: string, msgs: Message[]) => {
    setDelegateStore((prev) => {
      const entry = prev[delegateId] || {
        info: { id: delegateId, task, status: 'completed' as const, zIndex: 0, positionIndex: 0 },
        messages: [],
        isOpen: false,
      };
      return update({ ...prev, [delegateId]: { ...entry, messages: msgs } });
    });
  }, [update]);

  const hasDelegateMessages = useCallback((delegateId: string): boolean => {
    return !!delegateStoreRef.current[delegateId]?.messages?.length;
  }, []);

  const removeDelegate = useCallback((delegateId: string) => {
    setDelegateStore((prev) => {
      const entry = prev[delegateId];
      if (!entry) return prev;
      return update({ ...prev, [delegateId]: { ...entry, isOpen: false } });
    });
  }, [update]);

  return {
    delegates,
    delegateStatuses,
    delegateMessages,
    handleDelegateSnapshots,
    handleDelegateMessages,
    handleDelegateEvent,
    resetDelegates,
    restoreDelegates,
    bringToFront,
    openDelegate,
    setDelegateMessagesForId,
    hasDelegateMessages,
    removeDelegate,
  };
}
