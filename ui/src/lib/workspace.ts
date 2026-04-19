// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

export const WORKSPACE_LABEL_KEY = 'workspace';
export const WORKSPACE_LABEL_PREFIX = `${WORKSPACE_LABEL_KEY}=`;
export const WORKSPACE_STORAGE_KEY = 'dagu-selected-workspace';
export const LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY = 'dagu_cockpit_workspace';

const WORKSPACE_NAME_PATTERN = /^[A-Za-z0-9_-]+$/;

export function sanitizeWorkspaceName(name: string): string {
  const trimmed = name.trim();
  return isValidWorkspaceName(trimmed) ? trimmed : '';
}

export function isValidWorkspaceName(name: string): boolean {
  return WORKSPACE_NAME_PATTERN.test(name);
}

export function workspaceLabel(name: string): string | undefined {
  const sanitized = sanitizeWorkspaceName(name);
  if (!sanitized || !isValidWorkspaceName(sanitized)) {
    return undefined;
  }
  return `${WORKSPACE_LABEL_PREFIX}${sanitized}`;
}

export function isWorkspaceLabel(label: string): boolean {
  return label.toLowerCase().startsWith(WORKSPACE_LABEL_PREFIX);
}

export function withoutWorkspaceLabels(labels: string[] = []): string[] {
  return labels.filter((label) => !isWorkspaceLabel(label));
}

export function withWorkspaceLabel(
  labels: string[] = [],
  workspaceName: string
): string[] {
  const label = workspaceLabel(workspaceName);
  const filtered = withoutWorkspaceLabels(labels);
  return label ? [...filtered, label] : filtered;
}

export function getStoredWorkspaceName(): string {
  try {
    const stored = localStorage.getItem(WORKSPACE_STORAGE_KEY);
    if (stored !== null) {
      const sanitized = sanitizeWorkspaceName(stored);
      if (sanitized) {
        return sanitized;
      }
      localStorage.removeItem(WORKSPACE_STORAGE_KEY);
      return '';
    }
    const legacy = localStorage.getItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY);
    if (legacy !== null) {
      const sanitized = sanitizeWorkspaceName(legacy);
      if (sanitized) {
        localStorage.setItem(WORKSPACE_STORAGE_KEY, sanitized);
      }
      localStorage.removeItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY);
      return sanitized;
    }
  } catch {
    // Ignore storage access errors.
  }
  return '';
}

export function persistWorkspaceName(name: string): void {
  try {
    const sanitized = sanitizeWorkspaceName(name);
    if (sanitized) {
      localStorage.setItem(WORKSPACE_STORAGE_KEY, sanitized);
    } else {
      localStorage.removeItem(WORKSPACE_STORAGE_KEY);
    }
    localStorage.removeItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY);
  } catch {
    // Ignore storage access errors.
  }
}
