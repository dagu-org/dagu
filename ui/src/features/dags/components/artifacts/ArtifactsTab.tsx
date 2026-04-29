// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useContext } from 'react';

import { components } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { ArtifactBrowser } from '@/features/artifacts/components/ArtifactBrowser';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

type Props = {
  dagRun: DAGRunDetails;
  artifactEnabled?: boolean;
  className?: string;
  fillHeight?: boolean;
};

export default function ArtifactsTab({
  dagRun,
  artifactEnabled = false,
  className,
  fillHeight = false,
}: Props) {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const isSubDAGRun =
    !!dagRun.rootDAGRunId &&
    dagRun.rootDAGRunId !== dagRun.dagRunId &&
    !!dagRun.rootDAGRunName;

  const requestArtifactTree = React.useCallback(
    async (signal?: AbortSignal) => {
      if (isSubDAGRun) {
        return client.GET(
          '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/artifacts',
          {
            params: {
              path: {
                name: dagRun.rootDAGRunName!,
                dagRunId: dagRun.rootDAGRunId!,
                subDAGRunId: dagRun.dagRunId,
              },
              query: { remoteNode, recursive: true },
            },
            signal,
          }
        );
      }

      return client.GET('/dag-runs/{name}/{dagRunId}/artifacts', {
        params: {
          path: {
            name: dagRun.name,
            dagRunId: dagRun.dagRunId,
          },
          query: { remoteNode, recursive: true },
        },
        signal,
      });
    },
    [
      client,
      dagRun.dagRunId,
      dagRun.name,
      dagRun.rootDAGRunId,
      dagRun.rootDAGRunName,
      isSubDAGRun,
      remoteNode,
    ]
  );

  const requestArtifactPreview = React.useCallback(
    async (path: string) => {
      if (isSubDAGRun) {
        return client.GET(
          '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/artifacts/preview',
          {
            params: {
              path: {
                name: dagRun.rootDAGRunName!,
                dagRunId: dagRun.rootDAGRunId!,
                subDAGRunId: dagRun.dagRunId,
              },
              query: { remoteNode, path },
            },
          }
        );
      }

      return client.GET('/dag-runs/{name}/{dagRunId}/artifacts/preview', {
        params: {
          path: {
            name: dagRun.name,
            dagRunId: dagRun.dagRunId,
          },
          query: { remoteNode, path },
        },
      });
    },
    [
      client,
      dagRun.dagRunId,
      dagRun.name,
      dagRun.rootDAGRunId,
      dagRun.rootDAGRunName,
      isSubDAGRun,
      remoteNode,
    ]
  );

  const fetchArtifactDownload = React.useCallback(
    async (path: string, signal?: AbortSignal) => {
      const request = isSubDAGRun
        ? await client.GET(
            '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/artifacts/download',
            {
              params: {
                path: {
                  name: dagRun.rootDAGRunName!,
                  dagRunId: dagRun.rootDAGRunId!,
                  subDAGRunId: dagRun.dagRunId,
                },
                query: { remoteNode, path },
              },
              parseAs: 'blob',
              signal,
            }
          )
        : await client.GET('/dag-runs/{name}/{dagRunId}/artifacts/download', {
            params: {
              path: {
                name: dagRun.name,
                dagRunId: dagRun.dagRunId,
              },
              query: { remoteNode, path },
            },
            parseAs: 'blob',
            signal,
          });

      if (request.error || !request.data) {
        throw new Error(
          request.error?.message ||
            request.response.statusText ||
            'Download failed'
        );
      }

      return {
        data: request.data,
        response: request.response,
      };
    },
    [
      client,
      dagRun.dagRunId,
      dagRun.name,
      dagRun.rootDAGRunId,
      dagRun.rootDAGRunName,
      isSubDAGRun,
      remoteNode,
    ]
  );

  return (
    <ArtifactBrowser
      artifactEnabled={artifactEnabled}
      artifactsAvailable={!!dagRun.artifactsAvailable}
      disabledState="Artifact storage is not enabled for this DAG run."
      emptyState={
        <>
          Artifacts will appear here after a run writes files into
          <code className="mx-1 rounded bg-muted px-1.5 py-0.5 text-xs">
            DAG_RUN_ARTIFACTS_DIR
          </code>
          .
        </>
      }
      requestArtifactTree={requestArtifactTree}
      requestArtifactPreview={requestArtifactPreview}
      fetchArtifactDownload={fetchArtifactDownload}
      className={className}
      fillHeight={fillHeight}
    />
  );
}
