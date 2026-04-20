// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { WorkspaceScope } from '@/api/v1/schema';

export const WORKSPACE_LABEL_KEY = 'workspace';
export const WORKSPACE_LABEL_PREFIX = `${WORKSPACE_LABEL_KEY}=`;
export const WORKSPACE_STORAGE_KEY = 'dagu-selected-workspace';
export const WORKSPACE_SCOPE_STORAGE_KEY = 'dagu-selected-workspace-scope';
export const LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY = 'dagu_cockpit_workspace';

const WORKSPACE_NAME_PATTERN = /^[A-Za-z0-9_-]+$/;

export type WorkspaceSelection = {
  scope: WorkspaceScope;
  workspace?: string;
};

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

export function workspaceNameFromLabels(labels: string[] = []): string {
  let workspaceName = '';
  for (const label of labels) {
    if (!isWorkspaceLabel(label)) {
      continue;
    }
    const value = sanitizeWorkspaceName(
      label.slice(WORKSPACE_LABEL_PREFIX.length)
    );
    if (!value) {
      return '';
    }
    if (workspaceName && workspaceName !== value) {
      return '';
    }
    workspaceName = value;
  }
  return workspaceName;
}

export function defaultWorkspaceSelection(): WorkspaceSelection {
  return { scope: WorkspaceScope.accessible };
}

export function sanitizeWorkspaceSelection(
  selection?: Partial<WorkspaceSelection> | null
): WorkspaceSelection {
  if (!selection) {
    return defaultWorkspaceSelection();
  }
  if (selection.scope === WorkspaceScope.none) {
    return { scope: WorkspaceScope.none };
  }
  if (selection.scope === WorkspaceScope.workspace) {
    const workspace = sanitizeWorkspaceName(selection.workspace ?? '');
    return workspace
      ? { scope: WorkspaceScope.workspace, workspace }
      : defaultWorkspaceSelection();
  }
  return defaultWorkspaceSelection();
}

export function workspaceNameForSelection(
  selection?: Partial<WorkspaceSelection> | null
): string {
  const sanitized = sanitizeWorkspaceSelection(selection);
  return sanitized.scope === WorkspaceScope.workspace
    ? (sanitized.workspace ?? '')
    : '';
}

export function isAggregateWorkspaceSelection(
  selection?: Partial<WorkspaceSelection> | null
): boolean {
  return (
    sanitizeWorkspaceSelection(selection).scope === WorkspaceScope.accessible
  );
}

export function isMutableWorkspaceSelection(
  selection?: Partial<WorkspaceSelection> | null
): boolean {
  return (
    sanitizeWorkspaceSelection(selection).scope !== WorkspaceScope.accessible
  );
}

export function workspaceSelectionLabel(
  selection?: Partial<WorkspaceSelection> | null
): string {
  const sanitized = sanitizeWorkspaceSelection(selection);
  switch (sanitized.scope) {
    case WorkspaceScope.none:
      return 'No workspace';
    case WorkspaceScope.workspace:
      return sanitized.workspace ?? 'Workspace';
    case WorkspaceScope.accessible:
    default:
      return 'Accessible workspaces';
  }
}

export function workspaceSelectionKey(
  selection?: Partial<WorkspaceSelection> | null
): string {
  const sanitized = sanitizeWorkspaceSelection(selection);
  if (sanitized.scope === WorkspaceScope.workspace) {
    return `workspace:${sanitized.workspace}`;
  }
  return `scope:${sanitized.scope}`;
}

export function workspaceSelectionQuery(
  selection?: Partial<WorkspaceSelection> | null
): { workspaceScope: WorkspaceScope; workspace?: string } {
  const sanitized = sanitizeWorkspaceSelection(selection);
  if (sanitized.scope === WorkspaceScope.workspace) {
    return {
      workspaceScope: WorkspaceScope.workspace,
      workspace: sanitized.workspace,
    };
  }
  return { workspaceScope: sanitized.scope };
}

function parseStoredWorkspaceSelection(
  value: string
): WorkspaceSelection | null {
  try {
    const parsed = JSON.parse(value) as Partial<WorkspaceSelection>;
    const sanitized = sanitizeWorkspaceSelection(parsed);
    return sanitized;
  } catch {
    return null;
  }
}

export function getStoredWorkspaceSelection(): WorkspaceSelection {
  try {
    const stored = localStorage.getItem(WORKSPACE_SCOPE_STORAGE_KEY);
    if (stored !== null) {
      const parsed = parseStoredWorkspaceSelection(stored);
      if (parsed) {
        return parsed;
      }
      localStorage.removeItem(WORKSPACE_SCOPE_STORAGE_KEY);
    }

    const legacy = localStorage.getItem(WORKSPACE_STORAGE_KEY);
    if (legacy !== null) {
      const sanitized = sanitizeWorkspaceName(legacy);
      localStorage.removeItem(WORKSPACE_STORAGE_KEY);
      if (sanitized) {
        const selection = {
          scope: WorkspaceScope.workspace,
          workspace: sanitized,
        };
        persistWorkspaceSelection(selection);
        return selection;
      }
      return defaultWorkspaceSelection();
    }

    const cockpitLegacy = localStorage.getItem(
      LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY
    );
    if (cockpitLegacy !== null) {
      const sanitized = sanitizeWorkspaceName(cockpitLegacy);
      localStorage.removeItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY);
      if (sanitized) {
        const selection = {
          scope: WorkspaceScope.workspace,
          workspace: sanitized,
        };
        persistWorkspaceSelection(selection);
        return selection;
      }
    }
  } catch {
    // Ignore storage access errors.
  }
  return defaultWorkspaceSelection();
}

export function persistWorkspaceSelection(selection: WorkspaceSelection): void {
  try {
    const sanitized = sanitizeWorkspaceSelection(selection);
    localStorage.setItem(
      WORKSPACE_SCOPE_STORAGE_KEY,
      JSON.stringify(sanitized)
    );
    localStorage.removeItem(WORKSPACE_STORAGE_KEY);
    localStorage.removeItem(LEGACY_COCKPIT_WORKSPACE_STORAGE_KEY);
  } catch {
    // Ignore storage access errors.
  }
}

export function getStoredWorkspaceName(): string {
  return workspaceNameForSelection(getStoredWorkspaceSelection());
}

export function persistWorkspaceName(name: string): void {
  const sanitized = sanitizeWorkspaceName(name);
  persistWorkspaceSelection(
    sanitized
      ? { scope: WorkspaceScope.workspace, workspace: sanitized }
      : defaultWorkspaceSelection()
  );
}
