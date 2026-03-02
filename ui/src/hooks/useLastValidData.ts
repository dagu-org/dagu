import { useRef } from 'react';

/**
 * Caches the last non-null data value as anti-flash fallback.
 * Resets SYNCHRONOUSLY during render when resetKey changes —
 * no useEffect delay, zero frames of stale cross-node data.
 */
export function useLastValidData<T>(
  data: T | null | undefined,
  resetKey: string
): T | null {
  const cachedRef = useRef<T | null>(null);
  const prevKeyRef = useRef(resetKey);

  if (prevKeyRef.current !== resetKey) {
    prevKeyRef.current = resetKey;
    cachedRef.current = null;
  }

  if (data != null) {
    cachedRef.current = data as T;
  }

  return (data as T | null) ?? cachedRef.current;
}
