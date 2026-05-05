export function normalizeDocPathFromURL(path: string): string {
  return path.replace(/\.md$/i, '');
}
