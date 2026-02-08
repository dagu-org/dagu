/**
 * Checks if two tag arrays contain the same elements (order-independent).
 */
export const areTagsEqual = (a: string[], b: string[]): boolean => {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((tag, i) => tag === sortedB[i]);
};

/**
 * Formats a timezone offset (in seconds) as a string like "(+09:00)".
 */
export const formatTimezoneOffset = (
  tzOffsetInSec: number | undefined
): string => {
  if (tzOffsetInSec === undefined) return '';

  const offsetInMinutes = tzOffsetInSec / 60;
  const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
  const minutes = Math.abs(offsetInMinutes) % 60;
  const sign = offsetInMinutes >= 0 ? '+' : '-';
  const formattedHours = hours.toString().padStart(2, '0');
  const formattedMinutes = minutes.toString().padStart(2, '0');

  return `(${sign}${formattedHours}:${formattedMinutes})`;
};
