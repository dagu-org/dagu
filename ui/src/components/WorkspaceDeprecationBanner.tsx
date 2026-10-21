// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * Admin notice listing the workspace settings that stop applying when
 * workspaces are removed in v3.0.
 *
 * @module components
 */
import { WorkspaceDeprecationItemKind } from '@/api/v1/schema';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useRemoteNode } from '@/contexts/RemoteNodeContext';
import { useQuery } from '@/hooks/api';
import { I18nText } from '@/i18n/I18nText';
import { useI18n } from '@/i18n/I18nProvider';
import { AlertTriangle } from 'lucide-react';
import * as React from 'react';

const kindLabels: Record<WorkspaceDeprecationItemKind, string> = {
  [WorkspaceDeprecationItemKind.baseConfig]: 'Base config',
  [WorkspaceDeprecationItemKind.secret]: 'Secret',
  [WorkspaceDeprecationItemKind.profileEntry]: 'Profile entry',
  [WorkspaceDeprecationItemKind.notificationRoute]: 'Notification route',
  [WorkspaceDeprecationItemKind.incidentPolicy]: 'Incident policy',
  [WorkspaceDeprecationItemKind.configKey]: 'Config key',
  [WorkspaceDeprecationItemKind.user]: 'User',
  [WorkspaceDeprecationItemKind.apiKey]: 'API key',
  [WorkspaceDeprecationItemKind.wikiDirectory]: 'Wiki directory',
};

export function WorkspaceDeprecationBanner() {
  const { ts } = useI18n();
  const isAdmin = useIsAdmin();
  const remoteNode = useRemoteNode();
  const [expanded, setExpanded] = React.useState(false);
  const { data } = useQuery(
    '/workspaces/deprecation',
    isAdmin ? { params: { query: { remoteNode } } } : null
  );

  const items = data?.items ?? [];
  if (!isAdmin || items.length === 0) {
    return null;
  }

  return (
    <div
      role="status"
      className="bg-amber-50 dark:bg-amber-950 border-b border-amber-200 dark:border-amber-800 px-4 py-1.5 text-sm text-amber-800 dark:text-amber-200"
    >
      <div className="flex items-center justify-between gap-3">
        <span className="flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          <I18nText text="Workspaces are removed in v3.0. These workspace settings will stop applying, and users and API keys limited to workspaces will become viewers." />
        </span>
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          aria-expanded={expanded}
          className="shrink-0 underline hover:no-underline text-amber-900 dark:text-amber-100"
        >
          {expanded
            ? ts('Hide settings')
            : ts('Show {count} settings', { count: items.length })}
        </button>
      </div>
      {expanded ? (
        <ul className="mt-1 ml-6 space-y-0.5">
          {items.map((item) => (
            <li key={`${item.kind}:${item.name}`}>
              <span className="font-medium">{ts(kindLabels[item.kind])}</span>{' '}
              <code className="font-mono text-xs">{item.name}</code>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
