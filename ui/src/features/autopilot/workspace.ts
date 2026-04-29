// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { isWorkspaceLabel, workspaceLabel } from '@/lib/workspace';

export function workspaceTagForAutopilotSelection(
  selectedWorkspace: string
): string | undefined {
  return workspaceLabel(selectedWorkspace);
}

export function applySelectedWorkspaceToAutopilotTags(
  tags: string[],
  selectedWorkspace: string
): string[] {
  const workspaceTag = workspaceTagForAutopilotSelection(selectedWorkspace);
  if (!workspaceTag) {
    return tags;
  }
  const filtered = tags.filter((tag) => !isWorkspaceLabel(tag));
  return [...filtered, workspaceTag];
}

type WorkspaceTaggedAutopilot = {
  tags?: string[] | null;
};

export function autopilotMatchesSelectedWorkspace(
  item: WorkspaceTaggedAutopilot,
  selectedWorkspace: string
): boolean {
  if (!selectedWorkspace) {
    return true;
  }
  const workspaceTag = workspaceTagForAutopilotSelection(selectedWorkspace);
  if (!workspaceTag) {
    return false;
  }
  const normalizedWorkspaceTag = workspaceTag.toLowerCase();
  return (item.tags || []).some(
    (tag) => tag.toLowerCase() === normalizedWorkspaceTag
  );
}

export function filterAutopilotBySelectedWorkspace<
  T extends WorkspaceTaggedAutopilot,
>(items: T[], selectedWorkspace: string): T[] {
  return items.filter((item) =>
    autopilotMatchesSelectedWorkspace(item, selectedWorkspace)
  );
}
