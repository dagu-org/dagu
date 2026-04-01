import React, { useCallback, useRef, useState, type MutableRefObject } from 'react';
import { Input } from '@/components/ui/input';
import ConfirmModal from '@/ui/ConfirmModal';
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
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const createStateRef = useRef<'idle' | 'submitted' | 'cancelled'>('idle') as MutableRefObject<'idle' | 'submitted' | 'cancelled'>;

  const handleCreate = useCallback(() => {
    if (createStateRef.current !== 'idle') return;
    createStateRef.current = 'submitted';
    const name = inputRef.current?.value.trim().replace(/[^a-zA-Z0-9_]/g, '');
    if (name) {
      onCreate(name);
    }
    setIsCreating(false);
  }, [onCreate]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        handleCreate();
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        createStateRef.current = 'cancelled';
        setIsCreating(false);
      }
    },
    [handleCreate]
  );

  const selectedWs = workspaces.find((ws) => ws.name === selectedWorkspace);

  if (isCreating) {
    return (
      <div className="flex items-center gap-1">
        <Input
          ref={inputRef}
          autoFocus
          className="px-2 text-xs w-40"
          placeholder="Workspace name..."
          onKeyDown={handleKeyDown}
          onBlur={handleCreate}
        />
      </div>
    );
  }

  return (
    <>
    <div className="flex items-center gap-1">
      <Select value={selectedWorkspace || '__none__'} onValueChange={(v) => {
        if (v === '__new__') {
          createStateRef.current = 'idle';
          setIsCreating(true);
        } else if (v === '__none__') {
          onSelect('');
        } else {
          onSelect(v);
        }
      }}>
        <SelectTrigger className="text-xs w-40 py-1">
          <SelectValue placeholder="Select workspace" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">All workspaces</SelectItem>
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
          onClick={() => setDeleteTarget(selectedWs.id)}
          className="p-1 text-muted-foreground hover:text-destructive rounded"
          title="Delete workspace"
        >
          <Trash2 size={14} />
        </button>
      )}
    </div>
    <ConfirmModal
      title="Delete Workspace"
      buttonText="Delete"
      visible={!!deleteTarget}
      dismissModal={() => setDeleteTarget(null)}
      onSubmit={() => {
        if (deleteTarget) onDelete(deleteTarget);
        setDeleteTarget(null);
      }}
    >
      <p className="text-sm">Are you sure you want to delete this workspace? This action cannot be undone.</p>
    </ConfirmModal>
    </>
  );
}
