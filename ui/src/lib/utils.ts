import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Parses a tag string into its key and value components.
 * Supports both key-only tags ("production") and key=value tags ("env=prod").
 * For tags with multiple '=' characters, only the first '=' is used as delimiter.
 */
export function parseTagParts(tag: string): { key: string; value: string | null } {
  const eqIndex = tag.indexOf('=');
  if (eqIndex === -1) {
    return { key: tag, value: null };
  }
  return {
    key: tag.slice(0, eqIndex),
    value: tag.slice(eqIndex + 1),
  };
}

/**
 * Converts a step name to a valid Mermaid node ID by encoding
 * all non-ASCII-alphanumeric characters (including emojis, CJK characters,
 * and other Unicode) that could break Mermaid syntax.
 * Each character is encoded as its hex code point delimited by underscores
 * (e.g., 'ス' → 'u30b9_') to produce deterministic, collision-free IDs.
 */
export function toMermaidNodeId(stepName: string): string {
  return stepName.replace(
    /[^a-zA-Z0-9_]/gu,
    (ch) => `u${ch.codePointAt(0)!.toString(16)}_`,
  );
}
