import { useContext, useEffect, useRef, useState } from 'react';
import { useConfig } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { getAuthToken } from '@/lib/authHeaders';
import { Message, StreamResponse } from '../types';

function buildDelegateStreamUrl(baseUrl: string, delegateId: string, remoteNode: string): string {
  const url = new URL(`${baseUrl}/sessions/${delegateId}/stream`, window.location.origin);
  const token = getAuthToken();
  if (token) {
    url.searchParams.set('token', token);
  }
  url.searchParams.set('remoteNode', remoteNode);
  return url.toString();
}

export function useDelegateStream(delegateId: string) {
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isWorking, setIsWorking] = useState(true);
  const eventSourceRef = useRef<EventSource | null>(null);

  const baseUrl = `${config.apiURL}/agent`;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  useEffect(() => {
    const url = buildDelegateStreamUrl(baseUrl, delegateId, remoteNode);
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onmessage = (event) => {
      try {
        const data: StreamResponse = JSON.parse(event.data);

        if (data.messages) {
          setMessages((prev) => {
            const updated = [...prev];
            for (const msg of data.messages!) {
              const idx = updated.findIndex((m) => m.id === msg.id);
              if (idx !== -1) {
                updated[idx] = msg;
              } else {
                updated.push(msg);
              }
            }
            return updated;
          });
        }

        if (data.session_state) {
          setIsWorking(data.session_state.working);
        }
      } catch {
        // Transient SSE parse error, ignore
      }
    };

    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        setIsWorking(false);
      }
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [delegateId, baseUrl, remoteNode]);

  return { messages, isWorking };
}
