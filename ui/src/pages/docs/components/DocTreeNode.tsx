import { components, DocTreeNodeResponseType } from '@/api/v1/schema';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import {
  ChevronDown,
  ChevronRight,
  FileText,
  Folder,
  FolderOpen,
  MoreHorizontal,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react';
import React from 'react';

type DocTreeNodeResponse = components['schemas']['DocTreeNodeResponse'];

type ContextAction =
  | { type: 'create'; parentDir: string }
  | { type: 'rename'; docPath: string; title: string }
  | { type: 'delete'; docPath: string; title: string; isDir: boolean; hasChildren: boolean };

type Props = {
  node: DocTreeNodeResponse;
  depth: number;
  expandedIds: Set<string>;
  activeDocPath: string | null;
  onToggleExpand: (id: string) => void;
  onSelectFile: (docPath: string, title: string) => void;
  onContextAction: (action: ContextAction) => void;
  canWrite: boolean;
};

function DocTreeNode({
  node,
  depth,
  expandedIds,
  activeDocPath,
  onToggleExpand,
  onSelectFile,
  onContextAction,
  canWrite,
}: Props) {
  const isDir = node.type === DocTreeNodeResponseType.directory;
  const isExpanded = expandedIds.has(node.id);
  const isActive = !isDir && node.id === activeDocPath;
  const displayTitle = node.title || node.name;
  const hasChildren = !!(node.children && node.children.length > 0);

  const handleClick = () => {
    if (isDir) {
      onToggleExpand(node.id);
    } else {
      onSelectFile(node.id, displayTitle);
    }
  };

  return (
    <>
      <div
        className={cn(
          'flex items-center gap-1 py-1 pr-1 cursor-pointer group rounded-sm',
          'hover:bg-accent/50',
          isActive && 'bg-accent text-accent-foreground'
        )}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        onClick={handleClick}
      >
        {isDir ? (
          <>
            {isExpanded ? (
              <ChevronDown className="h-3 w-3 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronRight className="h-3 w-3 shrink-0 text-muted-foreground" />
            )}
            {isExpanded ? (
              <FolderOpen className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            ) : (
              <Folder className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            )}
          </>
        ) : (
          <>
            <span className="w-3 shrink-0" />
            <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          </>
        )}

        <span className="flex-1 text-sm truncate select-none">
          {displayTitle}
        </span>

        {canWrite && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="shrink-0 p-0.5 rounded-sm opacity-0 group-hover:opacity-100 hover:bg-muted-foreground/20 focus-visible:opacity-100"
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="h-3.5 w-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              {isDir ? (
                <>
                  <DropdownMenuItem
                    onClick={(e) => {
                      e.stopPropagation();
                      onContextAction({ type: 'create', parentDir: node.id });
                    }}
                  >
                    <Plus className="h-3.5 w-3.5 mr-2" />
                    New Document
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    disabled={hasChildren}
                    onClick={(e) => {
                      e.stopPropagation();
                      onContextAction({
                        type: 'delete',
                        docPath: node.id,
                        title: displayTitle,
                        isDir: true,
                        hasChildren,
                      });
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5 mr-2" />
                    Delete
                  </DropdownMenuItem>
                </>
              ) : (
                <>
                  <DropdownMenuItem
                    onClick={(e) => {
                      e.stopPropagation();
                      onContextAction({
                        type: 'rename',
                        docPath: node.id,
                        title: displayTitle,
                      });
                    }}
                  >
                    <Pencil className="h-3.5 w-3.5 mr-2" />
                    Rename
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={(e) => {
                      e.stopPropagation();
                      onContextAction({
                        type: 'delete',
                        docPath: node.id,
                        title: displayTitle,
                        isDir: false,
                        hasChildren: false,
                      });
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5 mr-2" />
                    Delete
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {isDir && isExpanded && node.children && (
        <>
          {node.children.map((child) => (
            <DocTreeNode
              key={child.id}
              node={child}
              depth={depth + 1}
              expandedIds={expandedIds}
              activeDocPath={activeDocPath}
              onToggleExpand={onToggleExpand}
              onSelectFile={onSelectFile}
              onContextAction={onContextAction}
              canWrite={canWrite}
            />
          ))}
        </>
      )}
    </>
  );
}

export default DocTreeNode;
export type { ContextAction };
