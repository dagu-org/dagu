import { cn } from '@/lib/utils';
import {
  ChevronDown,
  ChevronRight,
  FileText,
  Folder,
  FolderOpen,
  MoreHorizontal,
  Pencil,
  Trash2,
  FilePlus,
} from 'lucide-react';
import { components } from '@/api/v1/schema';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

type DocTreeNodeResponse = components['schemas']['DocTreeNodeResponse'];

interface DocTreeNodeProps {
  node: DocTreeNodeResponse;
  depth: number;
  activeDocId: string | null;
  expandedDirs: Set<string>;
  onToggleDir: (id: string) => void;
  onSelectDoc: (id: string, title: string) => void;
  onCreateDoc?: (parentDir: string) => void;
  onRenameDoc?: (id: string) => void;
  onDeleteDoc?: (id: string) => void;
  canWrite: boolean;
}

export function DocTreeNode({
  node,
  depth,
  activeDocId,
  expandedDirs,
  onToggleDir,
  onSelectDoc,
  onCreateDoc,
  onRenameDoc,
  onDeleteDoc,
  canWrite,
}: DocTreeNodeProps) {
  const isDir = node.type === 'directory';
  const isExpanded = expandedDirs.has(node.id);
  const isActive = !isDir && activeDocId === node.id;
  const title = node.title || node.name;

  return (
    <div>
      <div
        className={cn(
          'flex items-center group h-7 cursor-pointer text-sm hover:bg-accent/50 rounded-sm',
          isActive && 'bg-accent text-accent-foreground'
        )}
        style={{ paddingLeft: `${depth * 12 + 4}px` }}
        onClick={() => {
          if (isDir) {
            onToggleDir(node.id);
          } else {
            onSelectDoc(node.id, title);
          }
        }}
      >
        {isDir ? (
          <span className="flex items-center justify-center w-4 h-4 mr-1 shrink-0">
            {isExpanded ? (
              <ChevronDown className="w-3.5 h-3.5 text-muted-foreground" />
            ) : (
              <ChevronRight className="w-3.5 h-3.5 text-muted-foreground" />
            )}
          </span>
        ) : (
          <span className="w-4 h-4 mr-1 shrink-0" />
        )}
        {isDir ? (
          isExpanded ? (
            <FolderOpen className="w-4 h-4 mr-1.5 shrink-0 text-muted-foreground" />
          ) : (
            <Folder className="w-4 h-4 mr-1.5 shrink-0 text-muted-foreground" />
          )
        ) : (
          <FileText className="w-4 h-4 mr-1.5 shrink-0 text-muted-foreground" />
        )}
        <span className="flex-1 truncate select-none">{title}</span>
        {canWrite && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-accent mr-1 shrink-0"
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="w-3.5 h-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              {isDir && onCreateDoc && (
                <DropdownMenuItem onClick={() => onCreateDoc(node.id)}>
                  <FilePlus className="w-3.5 h-3.5 mr-2" />
                  New Document
                </DropdownMenuItem>
              )}
              {!isDir && onRenameDoc && (
                <DropdownMenuItem onClick={() => onRenameDoc(node.id)}>
                  <Pencil className="w-3.5 h-3.5 mr-2" />
                  Rename
                </DropdownMenuItem>
              )}
              {onDeleteDoc && (
                <DropdownMenuItem
                  onClick={() => onDeleteDoc(node.id)}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="w-3.5 h-3.5 mr-2" />
                  Delete
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
      {isDir && isExpanded && node.children && (
        <div>
          {node.children.map((child) => (
            <DocTreeNode
              key={child.id}
              node={child}
              depth={depth + 1}
              activeDocId={activeDocId}
              expandedDirs={expandedDirs}
              onToggleDir={onToggleDir}
              onSelectDoc={onSelectDoc}
              onCreateDoc={onCreateDoc}
              onRenameDoc={onRenameDoc}
              onDeleteDoc={onDeleteDoc}
              canWrite={canWrite}
            />
          ))}
        </div>
      )}
    </div>
  );
}
