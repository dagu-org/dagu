import { useEffect, useMemo, useRef, useState } from 'react';
import { buildAgentSessionTopic, sseManager } from '@/hooks/SSEManager';
import { StreamResponse } from '../types';

export interface SSECallbacks {
  onSnapshot: (snapshot: StreamResponse) => void;
  onDelegateSnapshot: (delegateId: string, snapshot: StreamResponse) => void;
  onNavigate: (path: string) => void;
  onPreConnect: () => void;
}

export interface AgentSSEStatus {
  isSessionLive: boolean;
  liveDelegateSessions: Record<string, boolean>;
}

export function useSSEConnection(
  sessionId: string | null,
  delegateSessionIds: string[],
  apiURL: string,
  remoteNode: string,
  callbacks: SSECallbacks
): AgentSSEStatus {
  const handledNavigateIdsRef = useRef<Set<string>>(new Set());
  const cbRef = useRef(callbacks);
  cbRef.current = callbacks;
  const [isSessionLive, setIsSessionLive] = useState(false);
  const [liveDelegateSessions, setLiveDelegateSessions] = useState<
    Record<string, boolean>
  >({});

  const delegateKey = useMemo(
    () => [...delegateSessionIds].sort().join('|'),
    [delegateSessionIds]
  );
  const stableDelegateSessionIds = useMemo(
    () => (delegateKey ? delegateKey.split('|') : []),
    [delegateKey]
  );

  useEffect(() => {
    handledNavigateIdsRef.current = new Set();
    setIsSessionLive(false);
  }, [sessionId]);

  useEffect(() => {
    setLiveDelegateSessions((prev) => {
      const next: Record<string, boolean> = {};
      for (const delegateId of stableDelegateSessionIds) {
        next[delegateId] = prev[delegateId] ?? false;
      }
      return next;
    });
  }, [stableDelegateSessionIds]);

  useEffect(() => {
    if (!sessionId) {
      setIsSessionLive(false);
      return;
    }

    cbRef.current.onPreConnect();

    return sseManager.subscribeTopic(
      buildAgentSessionTopic(sessionId),
      remoteNode,
      apiURL,
      {
        onData: (data) => {
          const snapshot = data as StreamResponse;
          cbRef.current.onSnapshot(snapshot);

          for (const msg of snapshot.messages ?? []) {
            if (
              msg.id &&
              msg.type === 'ui_action' &&
              msg.ui_action?.type === 'navigate' &&
              msg.ui_action.path &&
              !handledNavigateIdsRef.current.has(msg.id)
            ) {
              handledNavigateIdsRef.current.add(msg.id);
              cbRef.current.onNavigate(msg.ui_action.path);
            }
          }
        },
        onStateChange: (state) => {
          setIsSessionLive(state.isConnected && !state.shouldUseFallback);
        },
      }
    );
  }, [sessionId, remoteNode, apiURL]);

  useEffect(() => {
    if (!delegateKey) {
      setLiveDelegateSessions({});
      return;
    }

    const unsubscribes = stableDelegateSessionIds.map((delegateId) =>
      sseManager.subscribeTopic(
        buildAgentSessionTopic(delegateId),
        remoteNode,
        apiURL,
        {
          onData: (data) => {
            cbRef.current.onDelegateSnapshot(
              delegateId,
              data as StreamResponse
            );
          },
          onStateChange: (state) => {
            setLiveDelegateSessions((prev) => {
              const nextValue = state.isConnected && !state.shouldUseFallback;
              if (prev[delegateId] === nextValue) {
                return prev;
              }
              return {
                ...prev,
                [delegateId]: nextValue,
              };
            });
          },
        }
      )
    );

    return () => {
      for (const unsubscribe of unsubscribes) {
        unsubscribe();
      }
    };
  }, [delegateKey, stableDelegateSessionIds, remoteNode, apiURL]);

  return {
    isSessionLive,
    liveDelegateSessions,
  };
}
