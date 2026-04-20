// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Parses a label string into its key and value components.
 * Supports both key-only labels ("production") and key=value labels ("env=prod").
 * For labels with multiple '=' characters, only the first '=' is used as delimiter.
 */
export function parseLabelParts(label: string): {
  key: string;
  value: string | null;
} {
  const eqIndex = label.indexOf('=');
  if (eqIndex === -1) {
    return { key: label, value: null };
  }
  return {
    key: label.slice(0, eqIndex),
    value: label.slice(eqIndex + 1),
  };
}

export const parseTagParts = parseLabelParts;

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
    (ch) => `u${ch.codePointAt(0)!.toString(16)}_`
  );
}
