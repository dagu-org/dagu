// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useCallback, useRef, useState } from 'react';
import { Input } from '@/components/ui/input';
import ConfirmModal from '@/ui/ConfirmModal';
import { Briefcase, Plus, Trash2 } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { WorkspaceScope } from '@/api/v1/schema';
import type { components } from '@/api/v1/schema';
import { cn } from '@/lib/utils';
import {
  ACCESSIBLE_WORKSPACES_DISPLAY_NAME,
  defaultWorkspaceSelection,
  NO_WORKSPACE_DISPLAY_NAME,
  sanitizeWorkspaceName,
  sanitizeWorkspaceSelection,
  workspaceSelectionLabel,
  type WorkspaceSelection,
} from '@/lib/workspace';

type WorkspaceResponse = components['schemas']['WorkspaceResponse'];

const ACCESSIBLE_VALUE = '__accessible__';
const NONE_VALUE = '__none__';
const NEW_VALUE = '__new__';
const WORKSPACE_VALUE_PREFIX = 'workspace:';

interface Props {
  workspaces: WorkspaceResponse[];
  workspaceSelection?: WorkspaceSelection;
  onSelectScope?: (selection: WorkspaceSelection) => void;
  selectedWorkspace?: string;
  onSelect?: (name: string) => void;
  onCreate: (name: string) => void;
  onDelete: (id: string) => void;
  canWrite?: boolean;
  variant?: 'toolbar' | 'sidebar';
  collapsed?: boolean;
}

export function WorkspaceSelector({
  workspaces,
  workspaceSelection,
  onSelectScope,
  selectedWorkspace,
  onSelect,
  onCreate,
  onDelete,
  canWrite = true,
  variant = 'toolbar',
  collapsed = false,
}: Props): React.ReactElement {
  const [isCreating, setIsCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const createStateRef = useRef<'idle' | 'submitted' | 'cancelled'>('idle');
  const supportsScopedSelection = Boolean(onSelectScope);
  const selection = sanitizeWorkspaceSelection(
    workspaceSelection ??
      (selectedWorkspace
        ? {
            scope: WorkspaceScope.workspace,
            workspace: selectedWorkspace,
          }
        : defaultWorkspaceSelection())
  );
  const selectedValue =
    selection.scope === WorkspaceScope.workspace && selection.workspace
      ? `${WORKSPACE_VALUE_PREFIX}${selection.workspace}`
      : selection.scope === WorkspaceScope.none
        ? NONE_VALUE
        : ACCESSIBLE_VALUE;

  const handleCreate = useCallback(() => {
    if (createStateRef.current !== 'idle') return;
    createStateRef.current = 'submitted';
    const name = sanitizeWorkspaceName(inputRef.current?.value ?? '');
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

  const selectedWs =
    selection.scope === WorkspaceScope.workspace
      ? workspaces.find((ws) => ws.name === selection.workspace)
      : undefined;
  const handleSelect = (nextSelection: WorkspaceSelection) => {
    const sanitized = sanitizeWorkspaceSelection(nextSelection);
    onSelectScope?.(sanitized);
    if (!onSelectScope) {
      onSelect?.(
        sanitized.scope === WorkspaceScope.workspace
          ? (sanitized.workspace ?? '')
          : ''
      );
    }
  };

  if (isCreating) {
    return (
      <div
        className={cn(
          'flex items-center gap-1',
          variant === 'sidebar' && 'px-1'
        )}
      >
        <Input
          ref={inputRef}
          autoFocus
          className={cn(
            'px-2 text-xs',
            variant === 'sidebar' ? 'w-full h-9' : 'w-40'
          )}
          placeholder="Workspace name..."
          onKeyDown={handleKeyDown}
          onBlur={handleCreate}
        />
      </div>
    );
  }

  return (
    <>
      <div
        className={cn(
          'flex items-center gap-1',
          variant === 'sidebar' && 'px-1'
        )}
      >
        <Select
          value={selectedValue}
          onValueChange={(v) => {
            if (v === NEW_VALUE) {
              createStateRef.current = 'idle';
              setIsCreating(true);
            } else if (v === ACCESSIBLE_VALUE) {
              handleSelect({ scope: WorkspaceScope.accessible });
            } else if (v === NONE_VALUE) {
              handleSelect({ scope: WorkspaceScope.none });
            } else if (v.startsWith(WORKSPACE_VALUE_PREFIX)) {
              handleSelect({
                scope: WorkspaceScope.workspace,
                workspace: v.slice(WORKSPACE_VALUE_PREFIX.length),
              });
            } else {
              handleSelect({ scope: WorkspaceScope.accessible });
            }
          }}
        >
          <SelectTrigger
            aria-label="Workspace"
            className={cn(
              'text-xs',
              variant === 'sidebar'
                ? 'h-9 text-sidebar-foreground rounded-md bg-sidebar-hover border-sidebar-border hover:bg-sidebar-active'
                : 'w-40 py-1',
              collapsed &&
                'w-9 bg-transparent border-transparent hover:bg-sidebar-hover px-2 [&>svg:last-child]:hidden'
            )}
            style={
              variant === 'sidebar'
                ? {
                    transition:
                      'width 280ms cubic-bezier(0.4, 0, 0.2, 1), background-color 150ms ease, border-color 150ms ease, padding 280ms cubic-bezier(0.4, 0, 0.2, 1)',
                    width: collapsed ? '36px' : '100%',
                    paddingLeft: collapsed ? '9px' : '12px',
                    paddingRight: collapsed ? '9px' : '12px',
                  }
                : undefined
            }
            title={collapsed ? workspaceSelectionLabel(selection) : undefined}
          >
            {variant === 'sidebar' ? (
              <div className="flex items-center gap-2 min-w-0">
                <Briefcase
                  size={18}
                  className="text-sidebar-foreground flex-shrink-0"
                />
                <span
                  className="overflow-hidden whitespace-nowrap min-w-0"
                  style={{
                    transition:
                      'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), max-width 280ms cubic-bezier(0.4, 0, 0.2, 1)',
                    opacity: collapsed ? 0 : 1,
                    maxWidth: collapsed ? '0px' : '150px',
                  }}
                >
                  <SelectValue placeholder="Select workspace" />
                </span>
              </div>
            ) : (
              <SelectValue placeholder="Select workspace" />
            )}
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ACCESSIBLE_VALUE}>
              {ACCESSIBLE_WORKSPACES_DISPLAY_NAME}
            </SelectItem>
            {supportsScopedSelection && (
              <SelectItem value={NONE_VALUE}>
                {NO_WORKSPACE_DISPLAY_NAME}
              </SelectItem>
            )}
            {workspaces.map((ws) => (
              <SelectItem
                key={ws.id}
                value={`${WORKSPACE_VALUE_PREFIX}${ws.name}`}
              >
                {ws.name}
              </SelectItem>
            ))}
            {canWrite && !collapsed && (
              <SelectItem value={NEW_VALUE}>
                <span className="flex items-center gap-1 text-primary">
                  <Plus size={12} /> New workspace
                </span>
              </SelectItem>
            )}
          </SelectContent>
        </Select>
        {canWrite && !collapsed && selectedWs && (
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
        <p className="text-sm">
          Are you sure you want to delete this workspace? This action cannot be
          undone.
        </p>
      </ConfirmModal>
    </>
  );
}
