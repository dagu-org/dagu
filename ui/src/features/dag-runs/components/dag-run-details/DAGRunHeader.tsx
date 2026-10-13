// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  Calendar,
  Check,
  Copy,
  FileText,
  Hash,
  Link2,
  type LucideIcon,
  RefreshCw,
  Server,
  SlidersHorizontal,
  Terminal,
  Timer,
} from 'lucide-react';
import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v1/schema';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useRemoteNode } from '../../../../contexts/RemoteNodeContext';
import { useCopyFeedback } from '@/hooks/useCopyFeedback';
import dayjs from '../../../../lib/dayjs';
import RelativeTime from '@/components/ui/relative-time';
import StatusChip from '@/components/ui/status-chip';
import AutoRetryBadge from '../common/AutoRetryBadge';
import { DAGRunActions } from '../common';
import { buildDAGPageURL, buildDAGRunPageURL } from '../../lib/dagRunUrls';
import { I18nText } from '@/i18n/I18nText';
import { I18nProps } from '@/i18n/I18nProps';
import { useI18n } from '@/i18n/I18nProvider';
import { Button } from '@/components/ui/button';

interface DAGRunHeaderProps {
  dagRun: components['schemas']['DAGRunDetails'];
  rootDAGRun?: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
}

type MetaItemProps = {
  icon: LucideIcon;
  label: string;
  children: React.ReactNode;
};

function MetaItem({ icon: Icon, label, children }: MetaItemProps) {
  const { ts } = useI18n();
  return (
    <div className="flex min-w-0 items-center gap-1.5" title={ts(label)}>
      <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <dt className="sr-only">{ts(label)}</dt>
      <dd className="min-w-0 text-foreground">{children}</dd>
    </div>
  );
}

const DAGRunHeader: React.FC<DAGRunHeaderProps> = ({
  dagRun,
  rootDAGRun,
  refreshFn,
}) => {
  const navigate = useNavigate();
  const remoteNode = useRemoteNode();
  const config = useConfig();
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const { copied: linkCopied, copy: copyLink } = useCopyFeedback();
  const { copied: runIdCopied, copy: copyRunId } = useCopyFeedback();
  // Announces whichever copy happened last, even while the other's feedback
  // is still showing.
  const [lastCopy, setLastCopy] = React.useState<'link' | 'runId' | null>(null);

  const copyRunLink = () => {
    const basePrefix = config.basePath === '/' ? '' : (config.basePath ?? '');
    const runPath = buildDAGRunPageURL({
      rootDAGRunName: dagRun.rootDAGRunName,
      rootDAGRunId: dagRun.rootDAGRunId,
      remoteNode,
      subDAGRunId:
        dagRun.dagRunId !== dagRun.rootDAGRunId ? dagRun.dagRunId : undefined,
    });
    setLastCopy('link');
    void copyLink(`${window.location.origin}${basePrefix}${runPath}`);
  };

  function formatDuration(startDate: string, endDate: string): string {
    if (!startDate || !endDate) return '--';

    const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
  }

  function getDurationDisplay(startedAt: string, finishedAt: string): string {
    if (finishedAt) {
      return formatDuration(startedAt, finishedAt);
    }
    if (startedAt) {
      return formatDuration(startedAt, dayjs().toISOString());
    }
    return '--';
  }

  const handleRootDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      buildDAGRunPageURL({
        rootDAGRunName: dagRun.rootDAGRunName,
        rootDAGRunId: dagRun.rootDAGRunId,
        remoteNode,
      })
    );
  };

  const handleParentDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    if (dagRun.parentDAGRunId) {
      navigate(
        buildDAGRunPageURL({
          rootDAGRunName: dagRun.rootDAGRunName,
          rootDAGRunId: dagRun.rootDAGRunId,
          remoteNode,
          subDAGRunId: dagRun.parentDAGRunId,
        })
      );
    }
  };

  const handleRefresh = () => {
    setIsRefreshing(true);
    refreshFn();
    setTimeout(() => setIsRefreshing(false), 600);
  };

  // Add keyboard shortcut for refresh
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      // Check if user is typing in an input field
      const target = e.target as HTMLElement;
      if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.contentEditable === 'true' ||
        target.closest('.monaco-editor') ||
        target.closest('[role="textbox"]')
      ) {
        return;
      }

      // Check for 'r' key without modifiers
      if (
        e.key === 'r' &&
        !e.metaKey &&
        !e.ctrlKey &&
        !e.altKey &&
        !e.shiftKey
      ) {
        e.preventDefault();
        handleRefresh();
      }
    };

    window.addEventListener('keydown', handleKeyPress);
    return () => window.removeEventListener('keydown', handleKeyPress);
  }, [handleRefresh]);

  return (
    <div className="space-y-4">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground empty:hidden [&:not(:empty)]:mb-2">
            {dagRun.rootDAGRunId !== dagRun.dagRunId && (
              <>
                <span className="font-medium">
                  <I18nText text={'Root:'} />
                </span>
                <a
                  href={buildDAGRunPageURL({
                    rootDAGRunName: dagRun.rootDAGRunName,
                    rootDAGRunId: dagRun.rootDAGRunId,
                    remoteNode,
                  })}
                  onClick={handleRootDAGRunClick}
                  className="text-primary hover:text-primary hover:underline transition-colors font-medium"
                >
                  {dagRun.rootDAGRunName}
                </a>
                {rootDAGRun && (
                  <StatusChip status={rootDAGRun.status} size="sm">
                    {rootDAGRun.statusLabel || ''}
                  </StatusChip>
                )}
                <span className="text-muted-foreground mx-1">/</span>
              </>
            )}

            {dagRun.parentDAGRunName &&
              dagRun.parentDAGRunId &&
              dagRun.parentDAGRunName !== dagRun.rootDAGRunName &&
              dagRun.parentDAGRunName !== dagRun.name && (
                <>
                  <a
                    href={buildDAGRunPageURL({
                      rootDAGRunName: dagRun.rootDAGRunName,
                      rootDAGRunId: dagRun.rootDAGRunId,
                      remoteNode,
                      subDAGRunId: dagRun.parentDAGRunId,
                    })}
                    onClick={handleParentDAGRunClick}
                    className="text-primary hover:text-primary hover:underline transition-colors font-medium"
                  >
                    {dagRun.parentDAGRunName}
                  </a>
                  <span className="text-muted-foreground mx-1">/</span>
                </>
              )}
          </nav>

          <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
              <h1 className="min-w-0 break-all text-2xl font-semibold tracking-[-0.01em] text-foreground">
                {dagRun.name}
              </h1>
              {dagRun.status != Status.NotStarted && dagRun.status && (
                <StatusChip status={dagRun.status} size="md">
                  {dagRun.statusLabel || ''}
                </StatusChip>
              )}
              <AutoRetryBadge
                status={dagRun.status}
                count={dagRun.autoRetryCount}
                limit={dagRun.autoRetryLimit}
              />
            </div>
            {dagRun.status != Status.NotStarted && (
              <div className="flex shrink-0 items-center gap-2">
                <I18nProps>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={handleRefresh}
                    disabled={isRefreshing}
                    title="Refresh (R)"
                    aria-label="Refresh"
                  >
                    <RefreshCw className={isRefreshing ? 'animate-spin' : ''} />
                  </Button>
                </I18nProps>
                <DAGRunActions
                  dagRun={dagRun}
                  name={dagRun.name}
                  refresh={refreshFn}
                  displayMode="full"
                  hideUnavailable
                  isRootLevel={dagRun.rootDAGRunId === dagRun.dagRunId}
                />
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <dl className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm text-muted-foreground">
          {dagRun.status != Status.NotStarted && (
            <>
              <MetaItem icon={Calendar} label="Started">
                <RelativeTime timestamp={dagRun?.startedAt} fallback="--" />
              </MetaItem>
              <MetaItem icon={Timer} label="Duration">
                <span className="tabular-nums">
                  {getDurationDisplay(dagRun.startedAt, dagRun.finishedAt)}
                </span>
              </MetaItem>
            </>
          )}
          {dagRun.workerId && (
            <MetaItem icon={Server} label="Worker">
              <span className="font-mono text-xs">{dagRun.workerId}</span>
            </MetaItem>
          )}
          {dagRun.profileName && (
            <MetaItem icon={SlidersHorizontal} label="Profile">
              <span className="font-mono text-xs">{dagRun.profileName}</span>
            </MetaItem>
          )}
          <MetaItem icon={Hash} label="Run ID">
            <span className="inline-flex items-center gap-1">
              <code className="font-mono text-xs break-all">
                {dagRun.dagRunId}
              </code>
              <I18nProps>
                <button
                  type="button"
                  onClick={() => {
                    setLastCopy('runId');
                    void copyRunId(dagRun.dagRunId);
                  }}
                  className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  title={runIdCopied ? 'Copied' : 'Copy run ID'}
                  aria-label={runIdCopied ? 'Copied' : 'Copy run ID'}
                >
                  {runIdCopied ? (
                    <Check className="h-3.5 w-3.5 text-success" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                </button>
              </I18nProps>
            </span>
          </MetaItem>
        </dl>
        <div className="flex items-center gap-1 sm:ml-auto">
          {dagRun.sourceFileName && (
            <I18nProps>
              <a
                href={buildDAGPageURL({
                  fileName: dagRun.sourceFileName,
                  remoteNode,
                })}
                onClick={(e) => {
                  e.preventDefault();
                  navigate(
                    buildDAGPageURL({
                      fileName: dagRun.sourceFileName!,
                      remoteNode,
                    })
                  );
                }}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                title="View workflow definition"
              >
                <FileText className="h-3.5 w-3.5" />
                <span>
                  <I18nText text={'Definition'} />
                </span>
              </a>
            </I18nProps>
          )}
          <I18nProps>
            <button
              type="button"
              onClick={copyRunLink}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              title={linkCopied ? 'Link copied' : 'Copy link to this run'}
              aria-label={linkCopied ? 'Link copied' : 'Copy link to this run'}
            >
              {linkCopied ? (
                <Check className="h-3.5 w-3.5 text-success" />
              ) : (
                <Link2 className="h-3.5 w-3.5" />
              )}
              <span>
                {linkCopied ? (
                  <I18nText text={'Copied'} />
                ) : (
                  <I18nText text={'Copy link'} />
                )}
              </span>
            </button>
          </I18nProps>
          <span className="sr-only" aria-live="polite">
            {lastCopy === 'runId' && runIdCopied ? (
              <I18nText text={'Run ID copied to clipboard'} />
            ) : lastCopy === 'link' && linkCopied ? (
              <I18nText text={'Run link copied to clipboard'} />
            ) : (
              ''
            )}
          </span>
        </div>
      </div>

      {/* Parameters - Show if present */}
      {dagRun.params && (
        <div className="border-t border-border pt-4">
          <div className="flex items-center gap-2 mb-2">
            <Terminal className="h-4 w-4 text-muted-foreground" />
            <span className="text-xs font-semibold text-foreground/90">
              <I18nText text={'Parameters'} />
            </span>
          </div>
          <div className="bg-accent rounded-md px-3 py-1.5 font-mono text-xs text-foreground max-h-[120px] overflow-y-auto border">
            {dagRun.params}
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGRunHeader;
