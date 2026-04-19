// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useContext, useEffect, useMemo, useState } from 'react';
import {
  AlertCircle,
  Download,
  File,
  FileImage,
  FileText,
  Folder,
  RefreshCw,
} from 'lucide-react';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { cn } from '@/lib/utils';

type DAGRunSummary = components['schemas']['DAGRunSummary'];
type ArtifactTreeNode = components['schemas']['ArtifactTreeNode'];

interface Props {
  run: DAGRunSummary | null;
  isOpen: boolean;
  onClose: () => void;
}

function flattenNodes(nodes: ArtifactTreeNode[]): ArtifactTreeNode[] {
  const flat: ArtifactTreeNode[] = [];
  for (const node of nodes) {
    flat.push(node);
    if (node.children) {
      flat.push(...flattenNodes(node.children));
    }
  }
  return flat;
}

function formatBytes(size: number | undefined): string {
  if (size == null) {
    return '';
  }
  if (size < 1024) {
    return `${size} B`;
  }

  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = size / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[unitIndex]}`;
}

function fileIconFor(node: ArtifactTreeNode) {
  if (node.type === 'directory') {
    return Folder;
  }
  if (node.path.match(/\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i)) {
    return FileImage;
  }
  if (node.path.match(/\.(md|markdown|mdown|mkd|txt|log|json|ya?ml|csv)$/i)) {
    return FileText;
  }
  return File;
}

function ArtifactRow({
  node,
  depth,
  downloadingPath,
  onDownload,
}: {
  node: ArtifactTreeNode;
  depth: number;
  downloadingPath: string | null;
  onDownload: (node: ArtifactTreeNode) => void;
}) {
  const Icon = fileIconFor(node);
  const isDirectory = node.type === 'directory';
  const isDownloading = downloadingPath === node.path;

  return (
    <div>
      <div
        className={cn(
          'flex min-h-9 items-center gap-2 rounded-md px-2 py-1.5 text-sm',
          isDirectory ? 'bg-muted/30 text-foreground' : 'hover:bg-muted/50'
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
      >
        <Icon
          className={cn(
            'h-4 w-4 shrink-0',
            isDirectory ? 'text-primary' : 'text-muted-foreground'
          )}
        />
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium leading-tight">{node.name}</div>
          <div className="truncate text-[11px] text-muted-foreground">
            {node.path}
          </div>
        </div>
        {!isDirectory && (
          <>
            {node.size != null && (
              <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
                {formatBytes(node.size)}
              </span>
            )}
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => onDownload(node)}
              disabled={isDownloading}
              aria-label={`Download ${node.name}`}
              title={`Download ${node.name}`}
            >
              <Download
                className={cn('h-4 w-4', isDownloading && 'opacity-50')}
              />
            </Button>
          </>
        )}
      </div>
      {isDirectory && node.children && node.children.length > 0 && (
        <div className="space-y-1">
          {node.children.map((child) => (
            <ArtifactRow
              key={child.path}
              node={child}
              depth={depth + 1}
              downloadingPath={downloadingPath}
              onDownload={onDownload}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function ArtifactListModal({
  run,
  isOpen,
  onClose,
}: Props): React.ReactElement {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const runName = run?.name;
  const runId = run?.dagRunId;

  const [tree, setTree] = useState<ArtifactTreeNode[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [downloadingPath, setDownloadingPath] = useState<string | null>(null);

  const flatNodes = useMemo(() => flattenNodes(tree), [tree]);
  const fileCount = flatNodes.filter((node) => node.type === 'file').length;
  const totalSize = flatNodes.reduce(
    (sum, node) => sum + (node.type === 'file' ? node.size || 0 : 0),
    0
  );

  useEffect(() => {
    if (!isOpen || !runName || !runId) {
      setTree([]);
      setError(null);
      setDownloadError(null);
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    const controller = new AbortController();

    const fetchTree = async () => {
      setIsLoading(true);
      setError(null);
      setDownloadError(null);

      try {
        const request = await client.GET(
          '/dag-runs/{name}/{dagRunId}/artifacts',
          {
            params: {
              path: {
                name: runName,
                dagRunId: runId,
              },
              query: { remoteNode, recursive: true },
            },
            signal: controller.signal,
          }
        );

        if (cancelled) {
          return;
        }

        if (request.error) {
          setTree([]);
          setError(request.error.message || 'Failed to load artifacts');
          return;
        }

        setTree(request.data?.items ?? []);
      } catch (err: unknown) {
        if (cancelled || controller.signal.aborted) {
          return;
        }
        setTree([]);
        setError(
          err instanceof Error ? err.message : 'Failed to load artifacts'
        );
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void fetchTree();

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [client, isOpen, refreshKey, remoteNode, runId, runName]);

  const handleDownload = async (node: ArtifactTreeNode) => {
    if (!run || node.type !== 'file') {
      return;
    }

    setDownloadingPath(node.path);
    setDownloadError(null);

    try {
      const request = await client.GET(
        '/dag-runs/{name}/{dagRunId}/artifacts/download',
        {
          params: {
            path: {
              name: run.name,
              dagRunId: run.dagRunId,
            },
            query: { remoteNode, path: node.path },
          },
          parseAs: 'blob',
        }
      );

      if (request.error) {
        throw new Error(
          request.error.message ||
            request.response.statusText ||
            'Download failed'
        );
      }

      const blob = request.data;
      const objectUrl = URL.createObjectURL(blob);
      const link = document.createElement('a');
      const fileName =
        request.response.headers
          .get('Content-Disposition')
          ?.match(/filename="(.+)"/)?.[1] || node.name;

      link.href = objectUrl;
      link.download = fileName;
      link.click();
      URL.revokeObjectURL(objectUrl);
    } catch (err: unknown) {
      setDownloadError(err instanceof Error ? err.message : 'Download failed');
    } finally {
      setDownloadingPath(null);
    }
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[85vh] max-w-3xl overflow-hidden p-0">
        <DialogHeader className="border-b border-border px-5 py-4 pr-12">
          <DialogTitle className="flex items-center gap-2">
            <Folder className="h-5 w-5 text-primary" />
            Artifacts
          </DialogTitle>
          <DialogDescription className="truncate">
            {run ? `${run.name} / ${run.dagRunId}` : 'DAG run artifacts'}
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-center justify-between gap-3 border-b border-border px-5 py-3">
          <div className="min-w-0 text-sm">
            <span className="font-medium">{fileCount}</span>{' '}
            <span className="text-muted-foreground">
              {fileCount === 1 ? 'file' : 'files'}
            </span>
            {totalSize > 0 && (
              <span className="text-muted-foreground">
                {' '}
                · {formatBytes(totalSize)}
              </span>
            )}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setRefreshKey((current) => current + 1)}
            disabled={isLoading || !run}
          >
            <RefreshCw className={cn('h-4 w-4', isLoading && 'animate-spin')} />
            Refresh
          </Button>
        </div>

        <div className="max-h-[58vh] overflow-auto px-5 py-4">
          {error ? (
            <div className="flex items-start gap-2 rounded-md bg-destructive/5 px-3 py-3 text-sm text-destructive">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{error}</span>
            </div>
          ) : downloadError ? (
            <div className="mb-3 flex items-start gap-2 rounded-md bg-destructive/5 px-3 py-3 text-sm text-destructive">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{downloadError}</span>
            </div>
          ) : null}

          {isLoading ? (
            <div className="rounded-md border border-dashed border-border bg-muted/20 px-4 py-8 text-center text-sm text-muted-foreground">
              Loading artifacts...
            </div>
          ) : !error && tree.length === 0 ? (
            <div className="rounded-md border border-dashed border-border bg-muted/20 px-4 py-8 text-center text-sm text-muted-foreground">
              No artifact files were found for this run.
            </div>
          ) : !error ? (
            <div className="space-y-1">
              {tree.map((node) => (
                <ArtifactRow
                  key={node.path}
                  node={node}
                  depth={0}
                  downloadingPath={downloadingPath}
                  onDownload={handleDownload}
                />
              ))}
            </div>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
