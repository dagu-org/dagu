import { useCallback, useEffect, useRef, useState } from 'react';
import { useDAGSSE } from './useDAGSSE';

interface ConflictState {
  hasConflict: boolean;
  externalSpec: string | null;
}

interface UseDAGSpecWithConflictDetectionOptions {
  fileName: string;
  enabled?: boolean;
}

interface UseDAGSpecWithConflictDetectionResult {
  // SSE connection state
  isConnected: boolean;
  shouldUseFallback: boolean;
  error: Error | null;

  // Data from SSE
  dag: ReturnType<typeof useDAGSSE>['data'];

  // Spec-specific state
  spec: string | null;
  currentValue: string;
  setCurrentValue: (value: string) => void;
  hasUnsavedChanges: boolean;

  // Conflict detection
  conflict: ConflictState;
  resolveConflict: (action: 'discard' | 'ignore') => void;

  // Post-save tracking
  markAsSaved: (savedSpec: string) => void;
}

/**
 * Hook that wraps useDAGSSE and adds conflict detection for the DAG spec.
 * Detects when the spec changes externally while the user is editing.
 */
export function useDAGSpecWithConflictDetection({
  fileName,
  enabled = true,
}: UseDAGSpecWithConflictDetectionOptions): UseDAGSpecWithConflictDetectionResult {
  const sseResult = useDAGSSE(fileName, enabled);

  // Track local edits
  const [currentValue, setCurrentValueState] = useState<string>('');

  // Track the last known server spec (for change detection)
  const lastServerSpecRef = useRef<string | null>(null);

  // Track if user has started editing
  const hasUserEditedRef = useRef<boolean>(false);

  // Track recently saved spec (to ignore our own saves coming back via SSE)
  const recentlySavedSpecRef = useRef<string | null>(null);
  const recentlySavedTimeRef = useRef<number>(0);

  // Conflict state
  const [conflict, setConflict] = useState<ConflictState>({
    hasConflict: false,
    externalSpec: null,
  });

  // Reset all state when fileName changes (navigating to different DAG)
  useEffect(() => {
    lastServerSpecRef.current = null;
    hasUserEditedRef.current = false;
    recentlySavedSpecRef.current = null;
    recentlySavedTimeRef.current = 0;
    setCurrentValueState('');
    setConflict({ hasConflict: false, externalSpec: null });
  }, [fileName]);

  // Process incoming SSE data
  useEffect(() => {
    const incomingSpec = sseResult.data?.spec;
    if (typeof incomingSpec === 'undefined' || incomingSpec === null) {
      return;
    }

    // First load - initialize everything
    if (lastServerSpecRef.current === null) {
      lastServerSpecRef.current = incomingSpec;
      setCurrentValueState(incomingSpec);
      return;
    }

    // Check if this is our own save coming back (within 5 seconds)
    const timeSinceSave = Date.now() - recentlySavedTimeRef.current;
    if (
      recentlySavedSpecRef.current === incomingSpec &&
      timeSinceSave < 5000
    ) {
      // This is our own save, update refs silently
      lastServerSpecRef.current = incomingSpec;
      recentlySavedSpecRef.current = null;
      return;
    }

    // Check if server spec actually changed
    if (incomingSpec === lastServerSpecRef.current) {
      return; // No change
    }

    // Server spec changed externally
    const hasLocalChanges =
      hasUserEditedRef.current && currentValue !== lastServerSpecRef.current;

    if (hasLocalChanges) {
      // Conflict: user has unsaved edits AND external change occurred
      setConflict({
        hasConflict: true,
        externalSpec: incomingSpec,
      });
    } else {
      // No local edits - update silently
      lastServerSpecRef.current = incomingSpec;
      setCurrentValueState(incomingSpec);
      hasUserEditedRef.current = false;
    }
  }, [sseResult.data?.spec, currentValue]);

  // Handle user edits
  const setCurrentValue = useCallback((value: string) => {
    hasUserEditedRef.current = true;
    setCurrentValueState(value);
  }, []);

  // Resolve conflict
  const resolveConflict = useCallback(
    (action: 'discard' | 'ignore') => {
      if (action === 'discard') {
        // Discard local changes, accept external
        if (conflict.externalSpec) {
          lastServerSpecRef.current = conflict.externalSpec;
          setCurrentValueState(conflict.externalSpec);
          hasUserEditedRef.current = false;
        }
      } else {
        // Ignore external changes, keep local
        // Just update the server ref to prevent repeated dialogs
        if (conflict.externalSpec) {
          lastServerSpecRef.current = conflict.externalSpec;
        }
      }
      setConflict({ hasConflict: false, externalSpec: null });
    },
    [conflict.externalSpec]
  );

  // Called after successful save
  const markAsSaved = useCallback((savedSpec: string) => {
    recentlySavedSpecRef.current = savedSpec;
    recentlySavedTimeRef.current = Date.now();
    lastServerSpecRef.current = savedSpec;
    hasUserEditedRef.current = false;
  }, []);

  // Calculate unsaved changes
  const hasUnsavedChanges =
    lastServerSpecRef.current !== null &&
    currentValue !== lastServerSpecRef.current;

  return {
    isConnected: sseResult.isConnected,
    shouldUseFallback: sseResult.shouldUseFallback,
    error: sseResult.error,
    dag: sseResult.data,
    spec: sseResult.data?.spec ?? null,
    currentValue,
    setCurrentValue,
    hasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
  };
}
