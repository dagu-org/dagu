/**
 * JSON patch format used by the agent's patch tool
 */
export interface JsonPatch {
  old_string: string;
  new_string: string;
  operation?: string;
  path?: string;
}

/**
 * Check if content is a JSON patch object
 */
export function isJsonPatch(content: string): JsonPatch | null {
  if (!content || content.length < 10) return null;

  try {
    const parsed = JSON.parse(content.trim());
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      'old_string' in parsed &&
      'new_string' in parsed &&
      typeof parsed.old_string === 'string' &&
      typeof parsed.new_string === 'string'
    ) {
      return parsed as JsonPatch;
    }
  } catch {
    // Not valid JSON
  }
  return null;
}

export type DiffLineType = 'context' | 'addition' | 'deletion';

export interface DiffLine {
  type: DiffLineType;
  content: string;
}

/**
 * Compute diff lines from old and new strings
 */
export function computeDiffLines(oldStr: string, newStr: string): DiffLine[] {
  const oldLines = oldStr.split('\n');
  const newLines = newStr.split('\n');
  const result: DiffLine[] = [];

  const lcs = computeLCS(oldLines, newLines);
  let oldIdx = 0;
  let newIdx = 0;

  for (const common of lcs) {
    // Add deletions before this common line
    while (oldIdx < common.oldIndex) {
      result.push({ type: 'deletion', content: oldLines[oldIdx]! });
      oldIdx++;
    }

    // Add additions before this common line
    while (newIdx < common.newIndex) {
      result.push({ type: 'addition', content: newLines[newIdx]! });
      newIdx++;
    }

    // Add the common line as context
    result.push({ type: 'context', content: oldLines[oldIdx]! });
    oldIdx++;
    newIdx++;
  }

  // Add remaining deletions
  while (oldIdx < oldLines.length) {
    result.push({ type: 'deletion', content: oldLines[oldIdx]! });
    oldIdx++;
  }

  // Add remaining additions
  while (newIdx < newLines.length) {
    result.push({ type: 'addition', content: newLines[newIdx]! });
    newIdx++;
  }

  return result;
}

interface LCSItem {
  oldIndex: number;
  newIndex: number;
}

function computeLCS(oldLines: string[], newLines: string[]): LCSItem[] {
  const m = oldLines.length;
  const n = newLines.length;

  const dp: number[][] = Array(m + 1).fill(null).map(() => Array(n + 1).fill(0));

  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (oldLines[i - 1] === newLines[j - 1]) {
        dp[i]![j] = dp[i - 1]![j - 1]! + 1;
      } else {
        dp[i]![j] = Math.max(dp[i - 1]![j]!, dp[i]![j - 1]!);
      }
    }
  }

  const result: LCSItem[] = [];
  let i = m;
  let j = n;

  while (i > 0 && j > 0) {
    if (oldLines[i - 1] === newLines[j - 1]) {
      result.unshift({ oldIndex: i - 1, newIndex: j - 1 });
      i--;
      j--;
    } else if (dp[i - 1]![j]! > dp[i]![j - 1]!) {
      i--;
    } else {
      j--;
    }
  }

  return result;
}
