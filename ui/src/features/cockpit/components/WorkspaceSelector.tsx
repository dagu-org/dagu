import React, { useCallback, useRef, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { components } from '@/api/v1/schema';

type WorkspaceResponse = components['schemas']['WorkspaceResponse'];

interface Props {
  workspaces: WorkspaceResponse[];
  selectedWorkspace: string;
  onSelect: (name: string) => void;
  onCreate: (name: string) => void;
  onDelete: (id: string) => void;
  canWrite?: boolean;
}

export function WorkspaceSelector({
  workspaces,
  selectedWorkspace,
  onSelect,
  onCreate,
  onDelete,
  canWrite = true,
}: Props): React.ReactElement {
  const [isCreating, setIsCreating] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleCreate = useCallback(() => {
    const name = inputRef.current?.value.trim();
    if (name) {
      onCreate(name);
      setIsCreating(false);
    }
  }, [onCreate]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleCreate();
      if (e.key === 'Escape') setIsCreating(false);
    },
    [handleCreate]
  );

  const selectedWs = workspaces.find((ws) => ws.name === selectedWorkspace);

  if (isCreating) {
    return (
      <div className="flex items-center gap-1">
        <input
          ref={inputRef}
          autoFocus
          className="h-7 px-2 text-xs rounded-md border border-border bg-background w-36"
          placeholder="Workspace name..."
          onKeyDown={handleKeyDown}
          onBlur={handleCreate}
        />
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1">
      <Select value={selectedWorkspace || '__none__'} onValueChange={(v) => {
        if (v === '__new__') {
          setIsCreating(true);
        } else if (v === '__none__') {
          onSelect('');
        } else {
          onSelect(v);
        }
      }}>
        <SelectTrigger className="h-7 text-xs w-40">
          <SelectValue placeholder="Select workspace" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">
            <span className="text-muted-foreground">All workspaces</span>
          </SelectItem>
          {workspaces.map((ws) => (
            <SelectItem key={ws.id} value={ws.name}>{ws.name}</SelectItem>
          ))}
          {canWrite && (
            <SelectItem value="__new__">
              <span className="flex items-center gap-1 text-primary">
                <Plus size={12} /> New workspace
              </span>
            </SelectItem>
          )}
        </SelectContent>
      </Select>
      {canWrite && selectedWs && (
        <button
          onClick={() => onDelete(selectedWs.id)}
          className="p-1 text-muted-foreground hover:text-destructive rounded"
          title="Delete workspace"
        >
          <Trash2 size={14} />
        </button>
      )}
    </div>
  );
}
