// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';

import { useClient } from '@/hooks/api';
import { ArtifactBrowser } from '@/features/artifacts/components/ArtifactBrowser';

export function ControllerArtifactsTab({
  name,
  artifactDir,
  artifactsAvailable,
}: {
  name: string;
  artifactDir?: string | null;
  artifactsAvailable?: boolean | null;
}) {
  const client = useClient();

  const requestArtifactTree = React.useCallback(
    async (signal?: AbortSignal) =>
      client.GET('/controller/{name}/artifacts', {
        params: {
          path: { name },
          query: { recursive: true },
        },
        signal,
      }),
    [client, name]
  );

  const requestArtifactPreview = React.useCallback(
    async (path: string) =>
      client.GET('/controller/{name}/artifacts/preview', {
        params: {
          path: { name },
          query: { path },
        },
      }),
    [client, name]
  );

  const fetchArtifactDownload = React.useCallback(
    async (path: string, signal?: AbortSignal) => {
      const request = await client.GET('/controller/{name}/artifacts/download', {
        params: {
          path: { name },
          query: { path },
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
    [client, name]
  );

  return (
    <ArtifactBrowser
      artifactEnabled
      artifactsAvailable={!!artifactsAvailable}
      emptyState={
        <>
          Controller artifacts will appear here after workflows or operator
          actions write files into
          <code className="mx-1 rounded bg-muted px-1.5 py-0.5 text-xs">
            {artifactDir || '<controller-artifacts-dir>'}
          </code>
          .
        </>
      }
      requestArtifactTree={requestArtifactTree}
      requestArtifactPreview={requestArtifactPreview}
      fetchArtifactDownload={fetchArtifactDownload}
    />
  );
}
