import { useContext, useEffect, useRef, useState } from 'react';
import type { KeyedMutator } from 'swr';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import {
  appLiveManager,
  type AppLiveEvent,
  type LiveConnectionState,
} from './AppLiveManager';

const INITIAL_STATE: LiveConnectionState = {
  isConnected: false,
  isConnecting: false,
  shouldUseFallback: true,
  error: null,
};

interface LiveInvalidationOptions<T> {
  enabled?: boolean;
  matcher: (event: AppLiveEvent) => boolean;
  mutate: KeyedMutator<T>;
  debounceMs?: number;
}

export function liveFallbackOptions(
  liveState: LiveConnectionState,
  fallbackInterval: number = 2000
) {
  const liveActive = liveState.isConnected && !liveState.shouldUseFallback;
  const liveSettling = liveState.isConnecting && !liveState.shouldUseFallback;
  return {
    revalidateOnMount: true,
    revalidateIfStale: !liveActive && !liveSettling,
    revalidateOnFocus: !liveActive,
    refreshInterval: liveActive || liveSettling ? 0 : fallbackInterval,
  };
}

export const sseFallbackOptions = liveFallbackOptions;

export function useLiveInvalidation<T>({
  enabled = true,
  matcher,
  mutate,
  debounceMs = 200,
}: LiveInvalidationOptions<T>): LiveConnectionState {
  const appBarContext = useContext(AppBarContext);
  const config = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const [state, setState] = useState<LiveConnectionState>(INITIAL_STATE);

  const matcherRef = useRef(matcher);
  const mutateRef = useRef(mutate);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inFlightRef = useRef(false);
  const pendingRef = useRef(false);

  matcherRef.current = matcher;
  mutateRef.current = mutate;

  useEffect(() => {
    if (!enabled) {
      setState(INITIAL_STATE);
      return;
    }

    const runRevalidate = async (): Promise<void> => {
      if (inFlightRef.current) {
        pendingRef.current = true;
        return;
      }

      inFlightRef.current = true;
      try {
        await mutateRef.current();
      } catch {
        // The stream is advisory. Errors stay surfaced by the query itself.
      } finally {
        inFlightRef.current = false;
        if (pendingRef.current) {
          pendingRef.current = false;
          scheduleRevalidate(0);
        }
      }
    };

    const scheduleRevalidate = (delay: number = debounceMs): void => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
      debounceTimerRef.current = setTimeout(() => {
        debounceTimerRef.current = null;
        void runRevalidate();
      }, delay);
    };

    const unsubscribe = appLiveManager.subscribe(remoteNode, config.apiURL, {
      matches: (event) => matcherRef.current(event),
      onInvalidate: () => {
        scheduleRevalidate();
      },
      onStateChange: (nextState) => {
        setState(nextState);
      },
    });

    return () => {
      unsubscribe();
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
      inFlightRef.current = false;
      pendingRef.current = false;
    };
  }, [config.apiURL, debounceMs, enabled, remoteNode]);

  return state;
}

export function useLiveConnection(enabled: boolean = true): LiveConnectionState {
  const appBarContext = useContext(AppBarContext);
  const config = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const [state, setState] = useState<LiveConnectionState>(INITIAL_STATE);

  useEffect(() => {
    if (!enabled) {
      setState(INITIAL_STATE);
      return;
    }

    return appLiveManager.subscribe(remoteNode, config.apiURL, {
      matches: () => false,
      onInvalidate: () => {},
      onStateChange: (nextState) => {
        setState(nextState);
      },
    });
  }, [config.apiURL, enabled, remoteNode]);

  return state;
}

function isReset(event: AppLiveEvent): boolean {
  return event.type === 'reset';
}

export function useLiveDAGsList<T>(
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) =>
      isReset(event) ||
      event.type === 'dag.changed' ||
      event.type === 'dagrun.changed' ||
      event.type === 'queue.changed',
  });
}

export function useLiveDAG<T>(
  fileName: string,
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) =>
      isReset(event) ||
      event.type === 'dagrun.changed' ||
      (event.type === 'dag.changed' &&
        (!event.fileName || event.fileName === fileName)),
  });
}

export function useLiveDAGSpec<T>(
  fileName: string,
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) =>
      isReset(event) ||
      (event.type === 'dag.changed' &&
        (!event.fileName || event.fileName === fileName)),
  });
}

export function useLiveDAGRuns<T>(
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) => isReset(event) || event.type === 'dagrun.changed',
  });
}

export function useLiveQueues<T>(
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) => isReset(event) || event.type === 'queue.changed',
  });
}

export function useLiveDocTree<T>(
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) => isReset(event) || event.type === 'doc.changed',
  });
}

export function useLiveDoc<T>(
  docPath: string,
  mutate: KeyedMutator<T>,
  enabled: boolean = true
): LiveConnectionState {
  return useLiveInvalidation({
    enabled,
    mutate,
    matcher: (event) =>
      isReset(event) ||
      (event.type === 'doc.changed' &&
        (!event.path || event.path === docPath)),
  });
}
