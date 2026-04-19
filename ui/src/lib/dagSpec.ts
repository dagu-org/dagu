// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { isMap, isScalar, isSeq, parseDocument } from 'yaml';
import { withWorkspaceLabel, workspaceLabel } from './workspace';

function prependWorkspaceLabel(spec: string, label: string): string {
  return `labels:\n  - ${label}\n${spec}`;
}

function labelsFromNode(node: unknown): string[] {
  if (isSeq(node)) {
    return node.items
      .map((item) => (isScalar(item) ? String(item.value ?? '') : ''))
      .filter(Boolean);
  }

  if (isScalar(node) && node.value != null) {
    return String(node.value)
      .split(/[,\s]+/)
      .map((label) => label.trim())
      .filter(Boolean);
  }

  return [];
}

export function ensureWorkspaceLabelInDAGSpec(
  spec: string,
  workspaceName: string
): string {
  const label = workspaceLabel(workspaceName);
  if (!label) {
    return spec;
  }

  try {
    const doc = parseDocument(spec);
    if (doc.errors.length > 0 || !isMap(doc.contents)) {
      return prependWorkspaceLabel(spec, label);
    }

    const labels = labelsFromNode(doc.contents.get('labels', true));
    doc.set('labels', withWorkspaceLabel(labels, workspaceName));
    return doc.toString();
  } catch {
    return prependWorkspaceLabel(spec, label);
  }
}

export function defaultDAGSpec(workspaceName: string): string {
  return ensureWorkspaceLabelInDAGSpec(
    `steps:\n  - name: hello\n    command: echo hello\n`,
    workspaceName
  );
}
