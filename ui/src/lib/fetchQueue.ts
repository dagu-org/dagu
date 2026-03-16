/**
 * Connection-aware fetch queue that limits concurrent HTTP requests.
 *
 * Browsers enforce a 6-connection-per-origin limit for HTTP/1.1. EventSource
 * connections (SSE) hold slots permanently, leaving fewer for regular fetches.
 * This queue caps concurrent fetches so they never exhaust the remaining slots
 * and cause requests to stall as "pending."
 *
 * Budget: 2 slots reserved for EventSource (multiplexed SSE + agent SSE),
 * 4 slots available for fetch — this queue enforces that cap.
 *
 * Requests beyond the limit wait in a FIFO queue and proceed as slots free up.
 * No request is dropped.
 */

const MAX_CONCURRENT_FETCHES = 4;

let active = 0;
const waiting: Array<() => void> = [];

function acquire(): Promise<void> {
  if (active < MAX_CONCURRENT_FETCHES) {
    active++;
    return Promise.resolve();
  }
  return new Promise<void>((resolve) => {
    waiting.push(() => {
      active++;
      resolve();
    });
  });
}

function release(): void {
  active--;
  const next = waiting.shift();
  if (next) next();
}

export async function queuedFetch(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<Response> {
  await acquire();
  try {
    return await fetch(input, init);
  } finally {
    release();
  }
}
