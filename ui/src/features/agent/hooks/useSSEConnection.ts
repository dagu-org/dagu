import { useEffect, useMemo, useRef } from 'react';
import { buildAgentSessionTopic, sseManager } from '@/hooks/SSEManager';
import { StreamResponse } from '../types';

export interface SSECallbacks {
  onSnapshot: (snapshot: StreamResponse) => void;
  onDelegateSnapshot: (delegateId: string, snapshot: StreamResponse) => void;
  onNavigate: (path: string) => void;
  onPreConnect: () => void;
}

export function useSSEConnection(
  sessionId: string | null,
  delegateSessionIds: string[],
  apiURL: string,
  remoteNode: string,
  callbacks: SSECallbacks
) {
  const handledNavigateIdsRef = useRef<Set<string>>(new Set());
  const cbRef = useRef(callbacks);
  cbRef.current = callbacks;

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
  }, [sessionId]);

  useEffect(() => {
    if (!sessionId) {
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
        onStateChange: () => {
          // Agent UI handles reconnects implicitly through fresh snapshots.
        },
      }
    );
  }, [sessionId, remoteNode, apiURL]);

  useEffect(() => {
    if (!delegateKey) {
      return;
    }

    const unsubscribes = stableDelegateSessionIds.map((delegateId) =>
      sseManager.subscribeTopic(
        buildAgentSessionTopic(delegateId),
        remoteNode,
        apiURL,
        {
          onData: (data) => {
            cbRef.current.onDelegateSnapshot(delegateId, data as StreamResponse);
          },
          onStateChange: () => {
            // Delegate panels update from snapshots only.
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
}
