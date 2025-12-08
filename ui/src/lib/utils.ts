import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Converts a step name to a valid Mermaid node ID by replacing
 * characters that could break Mermaid syntax (spaces, dashes, parentheses).
 * Uses 'dagutmp' as the replacement to avoid collisions with step names
 * that already contain underscores.
 */
export function toMermaidNodeId(stepName: string): string {
  return stepName.replace(/[\s-()]/g, 'dagutmp');
}
