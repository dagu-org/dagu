// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { isWorkspaceLabel, workspaceLabel } from '@/lib/workspace';

export function workspaceTagForControllerSelection(
  selectedWorkspace: string
): string | undefined {
  return workspaceLabel(selectedWorkspace);
}

export function applySelectedWorkspaceToControllerLabels(
  labels: string[],
  selectedWorkspace: string
): string[] {
  const workspaceTag = workspaceTagForControllerSelection(selectedWorkspace);
  if (!workspaceTag) {
    return labels;
  }
  const filtered = labels.filter((label) => !isWorkspaceLabel(label));
  return [...filtered, workspaceTag];
}

type WorkspaceLabeledController = {
  labels?: string[] | null;
};

export function controllerMatchesSelectedWorkspace(
  item: WorkspaceLabeledController,
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
  return (item.labels || []).some(
    (label) => label.toLowerCase() === normalizedWorkspaceTag
  );
}

export function filterControllerBySelectedWorkspace<
  T extends WorkspaceLabeledController,
>(items: T[], selectedWorkspace: string): T[] {
  return items.filter((item) =>
    controllerMatchesSelectedWorkspace(item, selectedWorkspace)
  );
}
