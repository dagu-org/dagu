// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * Admin notice listing the workspace settings that stop applying when
 * workspaces are removed in v3.0. It stays a small icon until opened.
 *
 * @module components
 */
import { WorkspaceDeprecationItemKind } from '@/api/v1/schema';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useRemoteNode } from '@/contexts/RemoteNodeContext';
import { useQuery } from '@/hooks/api';
import { useI18n } from '@/i18n/I18nProvider';
import { AlertTriangle } from 'lucide-react';

const feedbackIssueURL = 'https://github.com/dagucloud/dagu/issues/3288';

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

export function WorkspaceDeprecationNotice() {
  const { ts } = useI18n();
  const isAdmin = useIsAdmin();
  const remoteNode = useRemoteNode();
  const { data } = useQuery(
    '/workspaces/deprecation',
    isAdmin ? { params: { query: { remoteNode } } } : null
  );

  const items = data?.items ?? [];
  if (!isAdmin || items.length === 0) {
    return null;
  }

  const title = ts('Workspaces are removed in v3.0');
  return (
    <Dialog>
      <DialogTrigger
        aria-label={title}
        title={title}
        className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-amber-600 hover:bg-sidebar-hover focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-ring dark:text-amber-400"
      >
        <AlertTriangle className="h-4 w-4" />
      </DialogTrigger>
      <DialogContent className="max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {ts(
              'These workspace settings will stop applying, and users and API keys limited to workspaces will become viewers.'
            )}
          </DialogDescription>
        </DialogHeader>
        <ul className="space-y-1 text-sm">
          {items.map((item) => (
            <li key={`${item.kind}:${item.name}`}>
              <span className="font-medium">{ts(kindLabels[item.kind])}</span>{' '}
              <code className="break-all font-mono text-xs">{item.name}</code>
            </li>
          ))}
        </ul>
        <a
          href={feedbackIssueURL}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-muted-foreground underline hover:no-underline"
        >
          {ts('Have feedback? Comment on the GitHub issue.')}
        </a>
      </DialogContent>
    </Dialog>
  );
}
