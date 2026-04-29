// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { isWorkspaceLabel, workspaceLabel } from '@/lib/workspace';

export function workspaceTagForControllerSelection(
  selectedWorkspace: string
): string | undefined {
  return workspaceLabel(selectedWorkspace);
}

export function applySelectedWorkspaceToControllerTags(
  tags: string[],
  selectedWorkspace: string
): string[] {
  const workspaceTag = workspaceTagForControllerSelection(selectedWorkspace);
  if (!workspaceTag) {
    return tags;
  }
  const filtered = tags.filter((tag) => !isWorkspaceLabel(tag));
  return [...filtered, workspaceTag];
}

type WorkspaceTaggedController = {
  tags?: string[] | null;
};

export function controllerMatchesSelectedWorkspace(
  item: WorkspaceTaggedController,
  selectedWorkspace: string
): boolean {
  if (!selectedWorkspace) {
    return true;
  }
  const workspaceTag = workspaceTagForControllerSelection(selectedWorkspace);
  if (!workspaceTag) {
    return false;
  }
  const normalizedWorkspaceTag = workspaceTag.toLowerCase();
  return (item.tags || []).some(
    (tag) => tag.toLowerCase() === normalizedWorkspaceTag
  );
}

export function filterControllerBySelectedWorkspace<
  T extends WorkspaceTaggedController,
>(items: T[], selectedWorkspace: string): T[] {
  return items.filter((item) =>
    controllerMatchesSelectedWorkspace(item, selectedWorkspace)
  );
}
