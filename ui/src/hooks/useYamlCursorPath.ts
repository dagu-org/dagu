import { useMemo } from 'react';
import { parseDocument, isMap, isSeq, isPair, isScalar, Pair, YAMLMap, YAMLSeq, Scalar, Document } from 'yaml';

export interface YamlPathSegment {
  key: string;
  isArrayIndex: boolean;
}

export interface YamlCursorInfo {
  path: string[];
  segments: YamlPathSegment[];
  currentKey: string | null;
  isInValue: boolean;
}

/**
 * Parses YAML content and returns the path at a given offset
 */
export function getYamlPathAtOffset(
  content: string,
  offset: number
): YamlCursorInfo {
  const defaultResult: YamlCursorInfo = {
    path: [],
    segments: [],
    currentKey: null,
    isInValue: false,
  };

  if (!content.trim()) {
    return defaultResult;
  }

  try {
    const doc = parseDocument(content, { keepSourceTokens: true });

    // Walk the document to find the node at offset
    const result = findNodeAtOffset(doc, offset);

    if (result) {
      return {
        path: result.path,
        segments: result.segments,
        currentKey: result.currentKey,
        isInValue: result.isInValue,
      };
    }

    // Fallback: parse by line analysis
    return parseByLineAnalysis(content, offset);
  } catch {
    // If YAML parsing fails, use line-based analysis
    return parseByLineAnalysis(content, offset);
  }
}

interface NodeSearchResult {
  path: string[];
  segments: YamlPathSegment[];
  currentKey: string | null;
  isInValue: boolean;
}

function findNodeAtOffset(
  doc: Document,
  offset: number
): NodeSearchResult | null {
  const path: string[] = [];
  const segments: YamlPathSegment[] = [];
  let currentKey: string | null = null;
  let isInValue = false;

  function walkNode(
    node: unknown,
    currentPath: string[],
    currentSegments: YamlPathSegment[]
  ): boolean {
    if (!node) return false;

    // Check if this node contains the offset
    const range = getNodeRange(node);
    if (!range) return false;

    const [start, , end] = range;
    if (offset < start || offset > end) return false;

    if (isMap(node)) {
      const map = node as YAMLMap;
      for (const item of map.items) {
        if (isPair(item)) {
          const pair = item as Pair;
          const keyNode = pair.key;
          const valueNode = pair.value;

          // Get key string
          const keyStr = isScalar(keyNode) ? String((keyNode as Scalar).value) : '';

          // Check if cursor is in the key
          const keyRange = getNodeRange(keyNode);
          if (keyRange && offset >= keyRange[0] && offset <= keyRange[2]) {
            path.length = 0;
            path.push(...currentPath, keyStr);
            segments.length = 0;
            segments.push(...currentSegments, { key: keyStr, isArrayIndex: false });
            currentKey = keyStr;
            isInValue = false;
            return true;
          }

          // Check if cursor is in the value
          const valueRange = getNodeRange(valueNode);
          if (valueRange && offset >= valueRange[0] && offset <= valueRange[2]) {
            const newPath = [...currentPath, keyStr];
            const newSegments = [...currentSegments, { key: keyStr, isArrayIndex: false }];

            // Recurse into value
            if (walkNode(valueNode, newPath, newSegments)) {
              return true;
            }

            // Cursor is in this value but not a nested structure
            path.length = 0;
            path.push(...newPath);
            segments.length = 0;
            segments.push(...newSegments);
            currentKey = keyStr;
            isInValue = true;
            return true;
          }

          // Check if cursor is between key and value (on the same line)
          if (keyRange && valueRange) {
            if (offset > keyRange[2] && offset < valueRange[0]) {
              path.length = 0;
              path.push(...currentPath, keyStr);
              segments.length = 0;
              segments.push(...currentSegments, { key: keyStr, isArrayIndex: false });
              currentKey = keyStr;
              isInValue = true;
              return true;
            }
          }
        }
      }
    }

    if (isSeq(node)) {
      const seq = node as YAMLSeq;
      for (let i = 0; i < seq.items.length; i++) {
        const item = seq.items[i];
        const itemRange = getNodeRange(item);

        if (itemRange && offset >= itemRange[0] && offset <= itemRange[2]) {
          const indexStr = String(i);
          const newPath = [...currentPath, indexStr];
          const newSegments = [...currentSegments, { key: indexStr, isArrayIndex: true }];

          // Recurse into item
          if (walkNode(item, newPath, newSegments)) {
            return true;
          }

          path.length = 0;
          path.push(...newPath);
          segments.length = 0;
          segments.push(...newSegments);
          currentKey = indexStr;
          isInValue = true;
          return true;
        }
      }
    }

    return false;
  }

  if (doc.contents) {
    walkNode(doc.contents, [], []);
  }

  if (path.length > 0) {
    return { path, segments, currentKey, isInValue };
  }

  return null;
}

function getNodeRange(node: unknown): [number, number, number] | null {
  if (!node || typeof node !== 'object') return null;

  const n = node as { range?: [number, number, number] };
  if (n.range && Array.isArray(n.range) && n.range.length >= 3) {
    return n.range;
  }

  return null;
}

/**
 * Fallback parser using line-based analysis
 */
function parseByLineAnalysis(content: string, offset: number): YamlCursorInfo {
  const lines = content.split('\n');
  let currentOffset = 0;
  let targetLine = 0;

  // Find which line the offset is on
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line) continue;
    const lineLength = line.length + 1; // +1 for newline
    if (currentOffset + lineLength > offset) {
      targetLine = i;
      break;
    }
    currentOffset += lineLength;
  }

  const path: string[] = [];
  const segments: YamlPathSegment[] = [];
  const indentStack: { indent: number; key: string; isArrayIndex: boolean }[] = [];

  for (let i = 0; i <= targetLine && i < lines.length; i++) {
    const line = lines[i];
    if (!line) continue;

    const trimmed = line.trimStart();
    if (!trimmed || trimmed.startsWith('#')) continue;

    const indent = line.length - trimmed.length;

    // Pop items from stack that have >= indent level
    while (indentStack.length > 0) {
      const last = indentStack[indentStack.length - 1];
      if (last && last.indent >= indent) {
        indentStack.pop();
      } else {
        break;
      }
    }

    // Check for array item
    const arrayMatch = trimmed.match(/^-\s*(.*)$/);
    if (arrayMatch) {
      // Count array items at this level
      let arrayIndex = 0;
      for (let j = i - 1; j >= 0; j--) {
        const prevLine = lines[j];
        if (!prevLine) continue;
        const prevTrimmed = prevLine.trimStart();
        const prevIndent = prevLine.length - prevTrimmed.length;

        if (prevIndent < indent) break;
        if (prevIndent === indent && prevTrimmed.startsWith('-')) {
          arrayIndex++;
        }
      }

      indentStack.push({ indent, key: String(arrayIndex), isArrayIndex: true });

      // Check for inline key after -
      const inlineContent = arrayMatch[1];
      if (inlineContent) {
        const keyMatch = inlineContent.match(/^([^:]+):\s*(.*)$/);
        if (keyMatch && keyMatch[1]) {
          indentStack.push({ indent: indent + 2, key: keyMatch[1].trim(), isArrayIndex: false });
        }
      }
      continue;
    }

    // Check for key: value
    const keyMatch = trimmed.match(/^([^:]+):\s*(.*)$/);
    if (keyMatch && keyMatch[1]) {
      const key = keyMatch[1].trim();
      indentStack.push({ indent, key, isArrayIndex: false });
    }
  }

  // Build path from stack
  for (const item of indentStack) {
    path.push(item.key);
    segments.push({ key: item.key, isArrayIndex: item.isArrayIndex });
  }

  const currentKey = path.length > 0 ? path[path.length - 1] ?? null : null;

  return {
    path,
    segments,
    currentKey,
    isInValue: true,
  };
}

/**
 * Hook to get YAML path from Monaco editor cursor position
 */
export function useYamlCursorPath(
  content: string,
  lineNumber: number,
  column: number
): YamlCursorInfo {
  const offset = useMemo(() => {
    if (!content) return 0;

    const lines = content.split('\n');
    let calculatedOffset = 0;

    for (let i = 0; i < lineNumber - 1 && i < lines.length; i++) {
      const line = lines[i];
      calculatedOffset += (line?.length ?? 0) + 1; // +1 for newline
    }

    const targetLine = lines[lineNumber - 1];
    calculatedOffset += Math.min(column - 1, targetLine?.length ?? 0);
    return calculatedOffset;
  }, [content, lineNumber, column]);

  const pathInfo = useMemo(() => {
    return getYamlPathAtOffset(content, offset);
  }, [content, offset]);

  return pathInfo;
}

/**
 * Utility to convert Monaco position to offset
 */
export function positionToOffset(
  content: string,
  lineNumber: number,
  column: number
): number {
  const lines = content.split('\n');
  let offset = 0;

  for (let i = 0; i < lineNumber - 1 && i < lines.length; i++) {
    const line = lines[i];
    offset += (line?.length ?? 0) + 1;
  }

  const targetLine = lines[lineNumber - 1];
  offset += Math.min(column - 1, targetLine?.length ?? 0);
  return offset;
}
