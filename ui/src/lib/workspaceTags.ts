const WORKSPACE_TAG_PREFIX = 'workspace=';

function normalizeTag(tag: string): string {
  return tag.trim().toLowerCase();
}

export function sanitizeWorkspaceName(name: string): string {
  return name.replace(/[^a-zA-Z0-9_-]/g, '').toLowerCase();
}

export function buildWorkspaceTag(
  workspaceName: string | null | undefined
): string | undefined {
  const safeName = sanitizeWorkspaceName(workspaceName ?? '');
  return safeName ? `${WORKSPACE_TAG_PREFIX}${safeName}` : undefined;
}

export function isWorkspaceTag(tag: string): boolean {
  return normalizeTag(tag).startsWith(WORKSPACE_TAG_PREFIX);
}

export function sanitizeFilterTags(tags: string[]): string[] {
  const seen = new Set<string>();
  const sanitized: string[] = [];

  for (const rawTag of tags) {
    const normalized = normalizeTag(rawTag);
    if (!normalized || isWorkspaceTag(normalized) || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    sanitized.push(normalized);
  }

  return sanitized;
}

export function filterWorkspaceTags(tags: string[]): string[] {
  return tags.filter((tag) => !isWorkspaceTag(tag));
}

export function appendWorkspaceTag(
  tags: string[],
  workspaceName: string
): string[] {
  const nextTags = sanitizeFilterTags(tags);
  const workspaceTag = buildWorkspaceTag(workspaceName);
  if (!workspaceTag) {
    return nextTags;
  }
  if (nextTags.includes(workspaceTag)) {
    return nextTags;
  }
  return [...nextTags, workspaceTag];
}

export function matchesWorkspaceSelection(
  tags: string[] | undefined | null,
  workspaceName: string
): boolean {
  const workspaceTag = buildWorkspaceTag(workspaceName);
  if (!workspaceTag) {
    return true;
  }
  return (tags ?? []).some((tag) => normalizeTag(tag) === workspaceTag);
}
