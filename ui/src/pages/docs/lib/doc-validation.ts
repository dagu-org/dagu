export const DOC_PATH_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_. -]*(\/[a-zA-Z0-9][a-zA-Z0-9_. -]*)*$/;

export function validateDocPath(path: string): { isValid: boolean; error?: string } {
  const trimmed = path.trim();
  if (!trimmed) {
    return { isValid: false, error: 'Path is required' };
  }
  if (trimmed.length > 256) {
    return { isValid: false, error: 'Path must be 256 characters or fewer' };
  }
  if (!DOC_PATH_PATTERN.test(trimmed)) {
    return {
      isValid: false,
      error: 'Invalid path. Use letters, numbers, underscores, dots, hyphens, and spaces. Use / for directories.',
    };
  }
  return { isValid: true };
}
