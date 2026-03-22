import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { isAbortLikeError } from '@/lib/requestTimeout';
import {
  type DAGRunDetails,
  type DAGRunDetailsRequestTarget,
  fetchDAGRunDetails,
} from './dagRunDetailsRequest';

function toError(
  error: unknown,
  fallbackMessage: string = 'Failed to load DAG run details'
): Error {
  return error instanceof Error ? error : new Error(fallbackMessage);
}

type UseBoundedDAGRunDetailsOptions = {
  target: DAGRunDetailsRequestTarget | null;
  enabled?: boolean;
  pollIntervalMs?: number;
};

type UseBoundedDAGRunDetailsResult = {
  data: DAGRunDetails | null;
  error: Error | null;
  isLoading: boolean;
  isValidating: boolean;
  refresh: () => Promise<void>;
};

export function useBoundedDAGRunDetails({
  target,
  enabled = true,
  pollIntervalMs = 0,
}: UseBoundedDAGRunDetailsOptions): UseBoundedDAGRunDetailsResult {
  const [data, setData] = useState<DAGRunDetails | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  const dataRef = useRef<DAGRunDetails | null>(null);
  const targetRef = useRef<DAGRunDetailsRequestTarget | null>(target);
  const enabledRef = useRef(enabled);
  const pollIntervalRef = useRef(pollIntervalMs);
  const inFlightRef = useRef<Promise<void> | null>(null);
  const pendingRef = useRef(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const controllerRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);

  const requestKey = useMemo(() => {
    if (!target) {
      return null;
    }
    return JSON.stringify(target);
  }, [target]);

  dataRef.current = data;
  targetRef.current = target;
  enabledRef.current = enabled;
  pollIntervalRef.current = pollIntervalMs;

  const clearPollTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const abortActiveRequest = useCallback(() => {
    if (controllerRef.current) {
      controllerRef.current.abort();
      controllerRef.current = null;
    }
  }, []);

  const schedulePoll = useCallback(
    (runFetch: () => Promise<void>) => {
      clearPollTimer();
      if (!enabledRef.current || pollIntervalRef.current <= 0) {
        return;
      }
      timerRef.current = setTimeout(() => {
        void runFetch();
      }, pollIntervalRef.current);
    },
    [clearPollTimer]
  );

  const runFetch = useCallback(async (): Promise<void> => {
    if (!enabledRef.current || targetRef.current == null) {
      return;
    }

    if (inFlightRef.current) {
      pendingRef.current = true;
      return inFlightRef.current;
    }

    const controller = new AbortController();
    controllerRef.current = controller;
    setError(null);

    if (dataRef.current == null) {
      setIsLoading(true);
    } else {
      setIsValidating(true);
    }

    const request = targetRef.current;
    const promise = (async () => {
      try {
        const next = await fetchDAGRunDetails(request, {
          signal: controller.signal,
        });
        if (controller.signal.aborted || !mountedRef.current) {
          return;
        }
        dataRef.current = next;
        setData(next);
        setError(null);
      } catch (fetchError) {
        if (
          (controller.signal.aborted && isAbortLikeError(fetchError)) ||
          !mountedRef.current
        ) {
          return;
        }
        setError(toError(fetchError));
      } finally {
        if (controllerRef.current === controller) {
          controllerRef.current = null;
        }
        inFlightRef.current = null;
        if (!mountedRef.current) {
          return;
        }
        setIsLoading(false);
        setIsValidating(false);

        if (pendingRef.current) {
          pendingRef.current = false;
          void runFetch();
          return;
        }

        if (
          enabledRef.current &&
          targetRef.current != null &&
          pollIntervalRef.current > 0
        ) {
          schedulePoll(runFetch);
        }
      }
    })();

    inFlightRef.current = promise;
    return promise;
  }, [schedulePoll]);

  useEffect(() => {
    if (!enabled || target == null) {
      clearPollTimer();
      abortActiveRequest();
      pendingRef.current = false;
      setIsLoading(false);
      setIsValidating(false);
      return;
    }

    clearPollTimer();
    abortActiveRequest();
    pendingRef.current = false;
    void runFetch();

    return () => {
      clearPollTimer();
      abortActiveRequest();
      pendingRef.current = false;
    };
  }, [
    abortActiveRequest,
    clearPollTimer,
    enabled,
    requestKey,
    runFetch,
  ]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      clearPollTimer();
      abortActiveRequest();
      pendingRef.current = false;
    };
  }, [abortActiveRequest, clearPollTimer]);

  const refresh = useCallback(async (): Promise<void> => {
    clearPollTimer();
    pendingRef.current = false;
    await runFetch();
  }, [clearPollTimer, runFetch]);

  return {
    data,
    error,
    isLoading,
    isValidating,
    refresh,
  };
}
