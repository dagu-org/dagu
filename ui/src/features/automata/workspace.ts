// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { isWorkspaceLabel, workspaceLabel } from '@/lib/workspace';

export function workspaceTagForAutomataSelection(
  selectedWorkspace: string
): string | undefined {
  return workspaceLabel(selectedWorkspace);
}

export function applySelectedWorkspaceToAutomataTags(
  tags: string[],
  selectedWorkspace: string
): string[] {
  const workspaceTag = workspaceTagForAutomataSelection(selectedWorkspace);
  if (!workspaceTag) {
    return tags;
  }
  const filtered = tags.filter((tag) => !isWorkspaceLabel(tag));
  return [...filtered, workspaceTag];
}

type WorkspaceTaggedAutomata = {
  tags?: string[] | null;
};

export function automataMatchesSelectedWorkspace(
  item: WorkspaceTaggedAutomata,
  selectedWorkspace: string
): boolean {
  if (!selectedWorkspace) {
    return true;
  }
  const workspaceTag = workspaceTagForAutomataSelection(selectedWorkspace);
  if (!workspaceTag) {
    return false;
  }
  const normalizedWorkspaceTag = workspaceTag.toLowerCase();
  return (item.tags || []).some(
    (tag) => tag.toLowerCase() === normalizedWorkspaceTag
  );
}

export function filterAutomataBySelectedWorkspace<
  T extends WorkspaceTaggedAutomata,
>(items: T[], selectedWorkspace: string): T[] {
  return items.filter((item) =>
    automataMatchesSelectedWorkspace(item, selectedWorkspace)
  );
}
