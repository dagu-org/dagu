import { useCallback, useEffect, useRef, useState } from 'react';
import { useDocSSE } from './useDocSSE';

interface ConflictState {
  hasConflict: boolean;
  externalContent: string | null;
}

interface UseDocContentWithConflictDetectionOptions {
  docPath: string;
  enabled?: boolean;
}

interface UseDocContentWithConflictDetectionResult {
  // SSE connection state
  isConnected: boolean;
  shouldUseFallback: boolean;
  error: Error | null;

  // Data from SSE
  doc: ReturnType<typeof useDocSSE>['data'];

  // Content-specific state
  content: string | null;
  title: string | null;
  currentValue: string;
  setCurrentValue: (value: string) => void;
  hasUnsavedChanges: boolean;

  // Conflict detection
  conflict: ConflictState;
  resolveConflict: (action: 'discard' | 'ignore') => void;

  // Post-save tracking
  markAsSaved: (savedContent: string) => void;
}

export function useDocContentWithConflictDetection({
  docPath,
  enabled = true,
}: UseDocContentWithConflictDetectionOptions): UseDocContentWithConflictDetectionResult {
  const sseResult = useDocSSE(docPath, enabled);

  const [currentValue, setCurrentValueState] = useState<string>('');
  const lastServerContentRef = useRef<string | null>(null);
  const hasUserEditedRef = useRef<boolean>(false);
  const pendingSaveContentRef = useRef<string | null>(null);

  const [conflict, setConflict] = useState<ConflictState>({
    hasConflict: false,
    externalContent: null,
  });

  // Reset all state when docPath changes
  useEffect(() => {
    lastServerContentRef.current = null;
    hasUserEditedRef.current = false;
    pendingSaveContentRef.current = null;
    setCurrentValueState('');
    setConflict({ hasConflict: false, externalContent: null });
  }, [docPath]);

  // Process incoming SSE data
  useEffect(() => {
    const incomingContent = sseResult.data?.content;
    if (typeof incomingContent === 'undefined' || incomingContent === null) {
      return;
    }

    // First load - initialize everything
    if (lastServerContentRef.current === null) {
      lastServerContentRef.current = incomingContent;
      setCurrentValueState(incomingContent);
      return;
    }

    // Check if this is our own save coming back
    if (pendingSaveContentRef.current === incomingContent) {
      lastServerContentRef.current = incomingContent;
      pendingSaveContentRef.current = null;
      return;
    }

    // Check if server content actually changed
    if (incomingContent === lastServerContentRef.current) {
      return;
    }

    // Server content changed externally
    const hasLocalChanges =
      hasUserEditedRef.current && currentValue !== lastServerContentRef.current;

    if (hasLocalChanges) {
      setConflict({
        hasConflict: true,
        externalContent: incomingContent,
      });
    } else {
      lastServerContentRef.current = incomingContent;
      setCurrentValueState(incomingContent);
      hasUserEditedRef.current = false;
    }
  }, [sseResult.data?.content, currentValue]);

  const setCurrentValue = useCallback((value: string) => {
    hasUserEditedRef.current = true;
    setCurrentValueState(value);
  }, []);

  const resolveConflict = useCallback(
    (action: 'discard' | 'ignore') => {
      if (action === 'discard') {
        if (conflict.externalContent) {
          lastServerContentRef.current = conflict.externalContent;
          setCurrentValueState(conflict.externalContent);
          hasUserEditedRef.current = false;
        }
      } else {
        if (conflict.externalContent) {
          lastServerContentRef.current = conflict.externalContent;
        }
      }
      setConflict({ hasConflict: false, externalContent: null });
    },
    [conflict.externalContent]
  );

  const markAsSaved = useCallback((savedContent: string) => {
    pendingSaveContentRef.current = savedContent;
    lastServerContentRef.current = savedContent;
    hasUserEditedRef.current = false;
  }, []);

  const hasUnsavedChanges =
    lastServerContentRef.current !== null &&
    currentValue !== lastServerContentRef.current;

  return {
    isConnected: sseResult.isConnected,
    shouldUseFallback: sseResult.shouldUseFallback,
    error: sseResult.error,
    doc: sseResult.data,
    content: sseResult.data?.content ?? null,
    title: sseResult.data?.title ?? null,
    currentValue,
    setCurrentValue,
    hasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
  };
}
