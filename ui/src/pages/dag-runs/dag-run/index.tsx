// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useCallback, useContext, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { RemoteNodeProvider } from '../../../contexts/RemoteNodeContext';
import { DAGRunDetailsContent } from '../../../features/dag-runs/components/dag-run-details';
import { DAGRunContext } from '../../../features/dag-runs/contexts/DAGRunContext';
import { matchesRequestedDAGRunDetails } from '../../../features/dag-runs/hooks/dagRunDetailsRequest';
import { useBoundedDAGRunDetails } from '../../../features/dag-runs/hooks/useBoundedDAGRunDetails';
import {
  LoadErrorState,
  NotFoundState,
  PageLoading,
} from '@/components/PageStates';
import { useI18n } from '@/i18n/I18nProvider';
import { I18nTemplate } from '@/i18n/I18nTemplate';
import { isNotFoundQueryError } from '@/lib/queryErrors';

type ErrorDisplayProps = {
  error: unknown;
  name: string | undefined;
  dagRunId: string | undefined;
  onRetry: () => void;
};

function ErrorDisplay({ error, name, dagRunId, onRetry }: ErrorDisplayProps) {
  const { ts } = useI18n();

  if (!isNotFoundQueryError(error)) {
    return (
      <LoadErrorState
        title={ts("Couldn't load this run")}
        error={error}
        onRetry={onRetry}
      />
    );
  }

  return (
    <NotFoundState
      title={ts('Run not found')}
      description={
        <I18nTemplate
          text="Run {runId} of {name} does not exist. It may have been dequeued or removed."
          values={{
            runId: <code>{dagRunId}</code>,
            name: <code>{name}</code>,
          }}
        />
      }
      action={{
        to: name ? `/dag-runs?name=${encodeURIComponent(name)}` : '/dag-runs',
        label: ts(name ? "Show this workflow's runs" : 'Show all runs'),
      }}
    />
  );
}

function DAGRunDetailsPage() {
  const { name, dagRunId = 'latest' } = useParams();
  const appBarContext = useContext(AppBarContext);

  const searchParams = new URLSearchParams(window.location.search);
  const subDAGRunId = searchParams.get('subDAGRunId');
  const parentDAGRunId = searchParams.get('dagRunId');
  const parentName = searchParams.get('dagRunName') || name;

  const canQuerySubDag = !!(subDAGRunId && parentDAGRunId && parentName);
  const queryRemoteNode = searchParams.get('remoteNode')?.trim();
  const appBarRemoteNode = appBarContext.selectedRemoteNode?.trim();
  const remoteNode = queryRemoteNode || appBarRemoteNode || 'local';
  const detailsTarget = canQuerySubDag
    ? {
        remoteNode,
        name: name || '',
        dagRunId: dagRunId || 'latest',
        parentName: parentName as string,
        parentDAGRunId: parentDAGRunId as string,
        subDAGRunId: subDAGRunId as string,
      }
    : name
      ? {
          remoteNode,
          name,
          dagRunId: dagRunId || 'latest',
        }
      : null;

  const {
    data: latestDetails,
    error,
    refresh,
  } = useBoundedDAGRunDetails({
    target: detailsTarget,
    enabled: detailsTarget !== null,
    pollIntervalMs: detailsTarget ? 2000 : 0,
  });

  const refreshFn = useCallback(() => {
    setTimeout(() => {
      void refresh();
    }, 500);
  }, [refresh]);

  const expectedDagRunId = subDAGRunId || dagRunId || 'latest';
  const dagRunDetails = matchesRequestedDAGRunDetails(
    latestDetails,
    expectedDagRunId,
    subDAGRunId ? undefined : name
  )
    ? latestDetails
    : null;
  const displayDAGRunId = subDAGRunId || dagRunId || '';

  function getDisplayName(): string {
    if (subDAGRunId) {
      return dagRunDetails?.name || parentName || '';
    }
    return name || '';
  }
  const displayName = getDisplayName();
  const { setTitle } = appBarContext;

  useEffect(() => {
    setTitle(displayName);
  }, [displayName, setTitle]);

  if (error && !dagRunDetails) {
    return (
      <ErrorDisplay
        error={error}
        name={name}
        dagRunId={dagRunId}
        onRetry={() => void refresh()}
      />
    );
  }

  if (!dagRunDetails) {
    return <PageLoading />;
  }

  return (
    <div className="flex h-full min-h-0 w-full flex-col px-4">
      <RemoteNodeProvider remoteNode={remoteNode}>
        <DAGRunContext.Provider
          value={{
            refresh: refreshFn,
            name: displayName,
            dagRunId: displayDAGRunId || '',
          }}
        >
          <DAGRunDetailsContent
            name={displayName}
            dagRun={dagRunDetails}
            refreshFn={refreshFn}
            dagRunId={displayDAGRunId}
            fillHeight
          />
        </DAGRunContext.Provider>
      </RemoteNodeProvider>
    </div>
  );
}

export default DAGRunDetailsPage;
