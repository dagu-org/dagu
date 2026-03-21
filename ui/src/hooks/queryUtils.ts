export function whenEnabled<T>(enabled: boolean, init: T): T | null {
  return enabled ? init : null;
}
