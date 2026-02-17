import { ReactElement, useCallback, useEffect, useRef } from 'react';

import { AlertCircle, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { useIsMobile } from '@/hooks/useIsMobile';

import { useAgentChatContext } from '../context/AgentChatContext';
import { useAgentChat } from '../hooks/useAgentChat';
import { useResizableDraggable } from '../hooks/useResizableDraggable';
import { SessionWithState, DAGContext } from '../types';
import { AgentChatModalHeader } from './AgentChatModalHeader';
import { ChatInput } from './ChatInput';
import { ChatMessages } from './ChatMessages';
import { DelegatePanel } from './DelegatePanel';
import { ResizeHandles } from './ResizeHandles';

function findLatestSession(
  sessions: SessionWithState[]
): SessionWithState | null {
  if (sessions.length === 0) return null;

  let latest: SessionWithState | null = null;
  for (const sess of sessions) {
    if (sess.session.parent_session_id) continue;
    if (
      !latest ||
      new Date(sess.session.updated_at) >
        new Date(latest.session.updated_at)
    ) {
      latest = sess;
    }
  }
  return latest;
}

export function AgentChatModal(): ReactElement | null {
  const { isOpen, isClosing, closeChat } = useAgentChatContext();
  const isMobile = useIsMobile();
  const {
    sessionId,
    messages,
    pendingUserMessage,
    sessionState,
    sessions,
    isWorking,
    error,
    answeredPrompts,
    setError,
    sendMessage,
    cancelSession,
    clearSession,
    clearError,
    fetchSessions,
    selectSession,
    respondToPrompt,
    delegates,
    delegateStatuses,
    delegateMessages,
    bringToFront,
    reopenDelegate,
    removeDelegate,
  } = useAgentChat();
  const { bounds, dragHandlers, resizeHandlers } = useResizableDraggable();

  const hasAutoSelectedRef = useRef(false);

  useEffect(() => {
    if (isOpen) {
      hasAutoSelectedRef.current = false;
      fetchSessions();
    }
  }, [isOpen, fetchSessions]);

  useEffect(() => {
    if (
      isOpen &&
      sessions.length > 0 &&
      !sessionId &&
      !hasAutoSelectedRef.current
    ) {
      hasAutoSelectedRef.current = true;
      const latest = findLatestSession(sessions);
      if (latest) {
        selectSession(latest.session.id).catch(() => {});
      }
    }
  }, [isOpen, sessions, sessionId, selectSession]);

  const handleSend = useCallback(
    (message: string, dagContexts?: DAGContext[], model?: string): void => {
      sendMessage(message, model, dagContexts).catch(() => {});
    },
    [sendMessage]
  );

  const handleCancel = useCallback((): void => {
    cancelSession().catch((err) =>
      setError(err instanceof Error ? err.message : 'Failed to cancel')
    );
  }, [cancelSession, setError]);

  const handleSelectSession = useCallback(
    (value: string): void => {
      if (value === 'new') {
        clearSession();
        return;
      }
      selectSession(value).catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to select session')
      );
    },
    [selectSession, clearSession, setError]
  );

  const handleOpenDelegate = useCallback((id: string) => {
    const info = delegateStatuses[id];
    if (info) {
      reopenDelegate(id, info.task);
    }
  }, [delegateStatuses, reopenDelegate]);

  if (!isOpen) return null;

  const errorBanner = error && (
    <div className="mx-3 mt-2 p-2 bg-destructive/10 border border-destructive/20 rounded-md flex items-start gap-2">
      <AlertCircle className="h-4 w-4 text-destructive flex-shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <p className="text-xs text-destructive">{error}</p>
      </div>
      <button
        onClick={clearError}
        className="text-destructive/60 hover:text-destructive flex-shrink-0"
      >
        <X className="h-3 w-3" />
      </button>
    </div>
  );

  const content = (
    <>
      <AgentChatModalHeader
        sessionId={sessionId}
        sessions={sessions}
        totalCost={sessionState?.total_cost}
        onSelectSession={handleSelectSession}
        onClearSession={clearSession}
        onClose={closeChat}
        dragHandlers={isMobile ? undefined : dragHandlers}
        isMobile={isMobile}
      />
      {errorBanner}
      <ChatMessages
        messages={messages}
        pendingUserMessage={pendingUserMessage}
        isWorking={isWorking}
        onPromptRespond={respondToPrompt}
        answeredPrompts={answeredPrompts}
        delegateStatuses={delegateStatuses}
        onOpenDelegate={handleOpenDelegate}
      />
      <ChatInput
        onSend={handleSend}
        onCancel={handleCancel}
        isWorking={isWorking}
        placeholder="Ask me to create a DAG, run a command..."
      />
    </>
  );

  // Mobile: fullscreen
  if (isMobile) {
    return (
      <div
        className={cn(
          'fixed inset-0 z-50',
          'flex flex-col',
          'bg-card dark:bg-zinc-950'
        )}
        style={{
          animation: isClosing
            ? 'agent-modal-out 250ms ease-in forwards'
            : 'agent-modal-in 400ms ease-out',
        }}
      >
        {content}
      </div>
    );
  }

  // Desktop: resizable/draggable window + delegate panels
  return (
    <>
      <div
        className={cn(
          'fixed z-50',
          'flex flex-col',
          'bg-card dark:bg-zinc-950 border border-border-strong rounded-lg overflow-hidden',
          'shadow-xl'
        )}
        style={{
          right: bounds.right,
          bottom: bounds.bottom,
          width: bounds.width,
          height: bounds.height,
          maxWidth: 'calc(100vw - 32px)',
          maxHeight: 'calc(100vh - 100px)',
          animation: isClosing
            ? 'agent-modal-out 250ms ease-in forwards'
            : 'agent-modal-in 400ms ease-out',
        }}
      >
        <ResizeHandles resizeHandlers={resizeHandlers} />
        {content}
      </div>
      {delegates.map((d, i) => (
        <DelegatePanel
          key={d.id}
          delegateId={d.id}
          task={d.task}
          status={d.status}
          zIndex={d.zIndex}
          index={i}
          messages={delegateMessages[d.id] || []}
          onClose={() => removeDelegate(d.id)}
          onBringToFront={() => bringToFront(d.id)}
        />
      ))}
    </>
  );
}
