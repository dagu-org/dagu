// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Check, FileText, Link2, RefreshCw } from 'lucide-react';
import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v1/schema';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useRemoteNode } from '../../../../contexts/RemoteNodeContext';
import { useCopyFeedback } from '@/hooks/useCopyFeedback';
import StatusChip from '@/components/ui/status-chip';
import AutoRetryBadge from '../common/AutoRetryBadge';
import { DAGRunActions } from '../common';
import { RunMetaLine } from '../common/RunMetaLine';
import { buildDAGPageURL, buildDAGRunPageURL } from '../../lib/dagRunUrls';
import { I18nText } from '@/i18n/I18nText';
import { I18nProps } from '@/i18n/I18nProps';
import { Button } from '@/components/ui/button';

interface DAGRunHeaderProps {
  dagRun: components['schemas']['DAGRunDetails'];
  rootDAGRun?: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
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
        <RunMetaLine
          dagRun={dagRun}
          onCopyRunId={() => {
            setLastCopy('runId');
            void copyRunId(dagRun.dagRunId);
          }}
          runIdCopied={runIdCopied}
        />
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
    </div>
  );
};

export default DAGRunHeader;
