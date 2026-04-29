// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  AlertCircle,
  Download,
  File,
  FileImage,
  FileText,
  Folder,
  FolderOpen,
  RefreshCw,
} from 'lucide-react';

import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { DocMarkdownPreview } from '@/components/ui/doc-markdown-preview';
import { cn } from '@/lib/utils';

type ArtifactTreeNode = components['schemas']['ArtifactTreeNode'];
type ArtifactPreviewResponse = components['schemas']['ArtifactPreviewResponse'];

type ArtifactTreeResponse = {
  data?: { items?: ArtifactTreeNode[] };
  error?: { message?: string };
};

type ArtifactPreviewRequest = {
  data?: ArtifactPreviewResponse;
  error?: { message?: string };
};

type ArtifactDownloadRequest = {
  data: Blob;
  error?: { message?: string };
  response: Response;
};

function collectDirectoryPaths(nodes: ArtifactTreeNode[]): string[] {
  const paths: string[] = [];
  for (const node of nodes) {
    if (node.type === 'directory') {
      paths.push(node.path);
      if (node.children) {
        paths.push(...collectDirectoryPaths(node.children));
      }
    }
  }
  return paths;
}

function findFirstFile(nodes: ArtifactTreeNode[]): ArtifactTreeNode | null {
  for (const node of nodes) {
    if (node.type === 'file') {
      return node;
    }
    if (node.children) {
      const child = findFirstFile(node.children);
      if (child) {
        return child;
      }
    }
  }
  return null;
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

function TreeNode({
  node,
  depth,
  openDirs,
  selectedPath,
  onToggleDir,
  onSelectFile,
}: {
  node: ArtifactTreeNode;
  depth: number;
  openDirs: Set<string>;
  selectedPath: string | null;
  onToggleDir: (path: string) => void;
  onSelectFile: (path: string) => void;
}) {
  const isDir = node.type === 'directory';
  const isOpen = isDir && openDirs.has(node.path);
  const isSelected = !isDir && selectedPath === node.path;

  const Icon = isDir
    ? isOpen
      ? FolderOpen
      : Folder
    : node.path.match(/\.(md|markdown|mdown|mkd)$/i)
      ? FileText
      : node.path.match(/\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i)
        ? FileImage
        : File;

  return (
    <div>
      <button
        type="button"
        onClick={() => {
          if (isDir) {
            onToggleDir(node.path);
            return;
          }
          onSelectFile(node.path);
        }}
        className={cn(
          'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors',
          isSelected
            ? 'bg-primary/10 text-primary'
            : 'text-foreground hover:bg-muted'
        )}
        style={{ paddingLeft: `${depth * 14 + 8}px` }}
      >
        <Icon className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{node.name}</span>
        {!isDir && node.size != null && (
          <span className="shrink-0 text-[11px] text-muted-foreground">
            {Intl.NumberFormat().format(node.size)}
          </span>
        )}
      </button>
      {isDir && isOpen && node.children && node.children.length > 0 && (
        <div className="space-y-0.5">
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              depth={depth + 1}
              openDirs={openDirs}
              selectedPath={selectedPath}
              onToggleDir={onToggleDir}
              onSelectFile={onSelectFile}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function ArtifactBrowser({
  artifactEnabled,
  artifactsAvailable,
  emptyState,
  disabledState,
  requestArtifactTree,
  requestArtifactPreview,
  fetchArtifactDownload,
  className,
  fillHeight = false,
}: {
  artifactEnabled?: boolean;
  artifactsAvailable: boolean;
  emptyState: React.ReactNode;
  disabledState?: React.ReactNode;
  requestArtifactTree: (signal?: AbortSignal) => Promise<ArtifactTreeResponse>;
  requestArtifactPreview: (path: string) => Promise<ArtifactPreviewRequest>;
  fetchArtifactDownload: (
    path: string,
    signal?: AbortSignal
  ) => Promise<ArtifactDownloadRequest>;
  className?: string;
  fillHeight?: boolean;
}) {
  const [tree, setTree] = useState<ArtifactTreeNode[]>([]);
  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<string | null>(null);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [preview, setPreview] = useState<ArtifactPreviewResponse | null>(null);
  const [previewVersion, setPreviewVersion] = useState(0);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [openDirs, setOpenDirs] = useState<Set<string>>(new Set());
  const treeRequestRef = useRef<{
    id: number;
    controller: AbortController | null;
  }>({ id: 0, controller: null });

  const allNodes = useMemo(() => flattenNodes(tree), [tree]);
  const selectedNode = useMemo(
    () => allNodes.find((node) => node.path === selectedPath) ?? null,
    [allNodes, selectedPath]
  );

  const fetchTree = async () => {
    const requestId = treeRequestRef.current.id + 1;
    treeRequestRef.current.controller?.abort();

    if (!artifactsAvailable) {
      treeRequestRef.current = { id: requestId, controller: null };
      setTree([]);
      setOpenDirs(new Set());
      setSelectedPath(null);
      setPreview(null);
      setTreeError(null);
      setTreeLoading(false);
      return;
    }

    const controller = new AbortController();
    treeRequestRef.current = { id: requestId, controller };

    const isCurrentRequest = () =>
      treeRequestRef.current.id === requestId &&
      treeRequestRef.current.controller === controller &&
      !controller.signal.aborted;

    setTreeLoading(true);
    setTreeError(null);
    try {
      const request = await requestArtifactTree(controller.signal);

      if (!isCurrentRequest()) {
        return;
      }

      if (request.error) {
        setTree([]);
        setOpenDirs(new Set());
        setSelectedPath(null);
        setPreview(null);
        setTreeError(request.error.message || 'Failed to load artifacts');
        return;
      }

      const items = request.data?.items ?? [];
      const nextNodes = flattenNodes(items);
      setTree(items);
      setOpenDirs(new Set(collectDirectoryPaths(items)));

      const firstFile = findFirstFile(items);
      if (!firstFile) {
        setSelectedPath(null);
        setPreview(null);
        return;
      }

      if (
        selectedPath &&
        nextNodes.some((node) => node.path === selectedPath)
      ) {
        setPreview(null);
        setPreviewVersion((current) => current + 1);
        return;
      }

      setPreview(null);
      setSelectedPath(firstFile.path);
    } catch (error: unknown) {
      if (controller.signal.aborted || !isCurrentRequest()) {
        return;
      }

      setTree([]);
      setOpenDirs(new Set());
      setSelectedPath(null);
      setPreview(null);
      setTreeError(
        error instanceof Error ? error.message : 'Failed to load artifacts'
      );
    } finally {
      if (
        treeRequestRef.current.id === requestId &&
        treeRequestRef.current.controller === controller
      ) {
        setTreeLoading(false);
        treeRequestRef.current = { id: requestId, controller: null };
      }
    }
  };

  useEffect(() => {
    void fetchTree();

    return () => {
      treeRequestRef.current.controller?.abort();
    };
  }, [artifactsAvailable, requestArtifactTree]);

  useEffect(() => {
    if (!selectedPath || !artifactsAvailable) {
      setPreview(null);
      setPreviewError(null);
      return;
    }

    let cancelled = false;
    setPreviewLoading(true);
    setPreviewError(null);

    const loadPreview = async () => {
      try {
        const request = await requestArtifactPreview(selectedPath);

        if (cancelled) {
          return;
        }

        if (request.error) {
          setPreview(null);
          setPreviewError(
            request.error.message || 'Failed to load artifact preview'
          );
          return;
        }

        setPreview(request.data ?? null);
      } catch (error: unknown) {
        if (cancelled) {
          return;
        }
        setPreview(null);
        setPreviewError(
          error instanceof Error
            ? error.message
            : 'Failed to load artifact preview'
        );
      } finally {
        if (!cancelled) {
          setPreviewLoading(false);
        }
      }
    };

    void loadPreview();

    return () => {
      cancelled = true;
    };
  }, [artifactsAvailable, previewVersion, requestArtifactPreview, selectedPath]);

  useEffect(() => {
    if (
      !preview ||
      preview.kind !== 'image' ||
      preview.tooLarge ||
      !selectedPath
    ) {
      setImageUrl(null);
      return;
    }

    let cancelled = false;
    let objectUrl = '';
    const controller = new AbortController();

    const loadImage = async () => {
      const request = await fetchArtifactDownload(
        selectedPath,
        controller.signal
      );
      if (cancelled) {
        return;
      }

      objectUrl = URL.createObjectURL(request.data);
      setImageUrl(objectUrl);
    };

    void loadImage().catch((error: unknown) => {
      if (cancelled) {
        return;
      }
      setPreviewError(
        error instanceof Error ? error.message : 'Failed to load image preview'
      );
    });

    return () => {
      cancelled = true;
      controller.abort();
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [fetchArtifactDownload, preview, selectedPath]);

  const handleDownload = async () => {
    if (!selectedPath) {
      return;
    }

    const request = await fetchArtifactDownload(selectedPath);
    const blob = request.data;
    const link = document.createElement('a');
    const objectUrl = URL.createObjectURL(blob);
    const fileName =
      request.response.headers
        .get('Content-Disposition')
        ?.match(/filename="(.+)"/)?.[1] ||
      selectedNode?.name ||
      'artifact';

    link.href = objectUrl;
    link.download = fileName;
    link.click();
    URL.revokeObjectURL(objectUrl);
  };

  if (!artifactEnabled && !artifactsAvailable) {
    return (
      <div className="rounded-lg border border-dashed border-border bg-muted/20 p-6 text-sm text-muted-foreground">
        {disabledState}
      </div>
    );
  }

  if (!artifactsAvailable) {
    return (
      <div className="rounded-lg border border-dashed border-border bg-muted/20 p-6 text-sm text-muted-foreground">
        {emptyState}
      </div>
    );
  }

  return (
    <div
      className={cn(
        'grid grid-cols-1 gap-4 xl:grid-cols-[320px_minmax(0,1fr)]',
        fillHeight && 'h-full min-h-0',
        className
      )}
    >
      <div
        className={cn(
          'rounded-lg border border-border bg-surface',
          fillHeight && 'flex min-h-0 flex-col overflow-hidden'
        )}
      >
        <div className="flex items-center justify-between border-b border-border px-3 py-2">
          <div>
            <p className="text-sm font-medium">Artifacts</p>
            <p className="text-xs text-muted-foreground">
              {tree.length === 0
                ? 'No files yet'
                : `${allNodes.filter((node) => node.type === 'file').length} files`}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => {
              void fetchTree();
            }}
            title="Reload artifacts"
          >
            <RefreshCw
              className={cn('h-4 w-4', treeLoading && 'animate-spin')}
            />
          </Button>
        </div>

        <div
          className={cn(
            'overflow-auto p-2',
            fillHeight ? 'min-h-0 flex-1' : 'max-h-[34rem]'
          )}
        >
          {treeLoading ? (
            <div className="px-2 py-6 text-sm text-muted-foreground">
              Loading artifacts...
            </div>
          ) : treeError ? (
            <div className="flex items-start gap-2 rounded-md bg-destructive/5 px-3 py-3 text-sm text-destructive">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{treeError}</span>
            </div>
          ) : tree.length === 0 ? (
            <div className="px-2 py-6 text-sm text-muted-foreground">
              No artifacts have been written yet.
            </div>
          ) : (
            <div className="space-y-0.5">
              {tree.map((node) => (
                <TreeNode
                  key={node.path}
                  node={node}
                  depth={0}
                  openDirs={openDirs}
                  selectedPath={selectedPath}
                  onToggleDir={(path) => {
                    setOpenDirs((current) => {
                      const next = new Set(current);
                      if (next.has(path)) {
                        next.delete(path);
                      } else {
                        next.add(path);
                      }
                      return next;
                    });
                  }}
                  onSelectFile={setSelectedPath}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <div
        className={cn(
          'rounded-lg border border-border bg-background',
          fillHeight && 'flex min-h-0 flex-col overflow-hidden'
        )}
      >
        <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">
              {selectedNode?.name || 'Select an artifact'}
            </p>
            <p className="truncate text-xs text-muted-foreground">
              {selectedPath || 'Choose a file from the left panel'}
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            disabled={!selectedPath || selectedNode?.type !== 'file'}
            onClick={() => {
              void handleDownload().catch((error: unknown) => {
                setPreviewError(
                  error instanceof Error ? error.message : 'Download failed'
                );
              });
            }}
          >
            <Download className="h-4 w-4" />
            Download
          </Button>
        </div>

        <div
          className={cn(
            'overflow-auto p-4',
            fillHeight ? 'min-h-0 flex-1' : 'max-h-[34rem]'
          )}
        >
          {!selectedPath ? (
            <div className="text-sm text-muted-foreground">
              Select a file to preview it.
            </div>
          ) : previewLoading ? (
            <div className="text-sm text-muted-foreground">
              Loading preview...
            </div>
          ) : previewError ? (
            <div className="flex items-start gap-2 rounded-md bg-destructive/5 px-3 py-3 text-sm text-destructive">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{previewError}</span>
            </div>
          ) : !preview ? (
            <div className="text-sm text-muted-foreground">
              Preview unavailable.
            </div>
          ) : preview.tooLarge ? (
            <div className="rounded-md border border-dashed border-border bg-muted/20 p-6">
              <p className="text-sm font-medium">Preview unavailable</p>
              <p className="mt-1 text-sm text-muted-foreground">
                This artifact is too large to render inline. Download it to
                inspect the contents.
              </p>
              <dl className="mt-4 space-y-1 text-xs text-muted-foreground">
                <div>
                  <dt className="inline font-medium text-foreground">MIME:</dt>{' '}
                  <dd className="inline">{preview.mimeType}</dd>
                </div>
                <div>
                  <dt className="inline font-medium text-foreground">Size:</dt>{' '}
                  <dd className="inline">
                    {Intl.NumberFormat().format(preview.size)} bytes
                  </dd>
                </div>
              </dl>
            </div>
          ) : preview.kind === 'markdown' ? (
            <DocMarkdownPreview content={preview.content} />
          ) : preview.kind === 'text' ? (
            <pre className="overflow-auto rounded-md border border-border bg-muted/20 p-4 text-sm leading-6 whitespace-pre-wrap">
              {preview.content || ''}
            </pre>
          ) : preview.kind === 'image' ? (
            imageUrl ? (
              <img
                src={imageUrl}
                alt={preview.name}
                className={cn(
                  'max-w-full rounded-md border border-border object-contain',
                  fillHeight ? 'max-h-full' : 'max-h-[40rem]'
                )}
              />
            ) : (
              <div className="text-sm text-muted-foreground">
                Loading image preview...
              </div>
            )
          ) : (
            <div className="rounded-md border border-dashed border-border bg-muted/20 p-6">
              <p className="text-sm font-medium">Binary artifact</p>
              <p className="mt-1 text-sm text-muted-foreground">
                This file can’t be rendered inline. Download it to inspect the
                contents.
              </p>
              <dl className="mt-4 space-y-1 text-xs text-muted-foreground">
                <div>
                  <dt className="inline font-medium text-foreground">MIME:</dt>{' '}
                  <dd className="inline">{preview.mimeType}</dd>
                </div>
                <div>
                  <dt className="inline font-medium text-foreground">Size:</dt>{' '}
                  <dd className="inline">
                    {Intl.NumberFormat().format(preview.size)} bytes
                  </dd>
                </div>
              </dl>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
