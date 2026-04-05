export function whenEnabled<T>(enabled: boolean, init: T): T | null {
  return enabled ? init : null;
}

export function optionalPositiveInt(value: unknown): number | undefined {
  const parsed =
    typeof value === 'number'
      ? value
      : typeof value === 'string' && value.trim() !== ''
        ? Number(value)
        : Number.NaN;

  if (!Number.isInteger(parsed) || parsed < 1) {
    return undefined;
  }

  return parsed;
}
