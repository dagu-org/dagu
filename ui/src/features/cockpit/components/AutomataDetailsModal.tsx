// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { Copy, Pencil, RotateCcw, Trash2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { AutomataDetailSurface } from '@/features/automata/components/AutomataDetailSurface';
import { useAutomataDetailController } from '@/features/automata/hooks/useAutomataDetail';
import { cn } from '@/lib/utils';
import { shouldIgnoreKeyboardShortcuts } from '@/lib/keyboard-shortcuts';

const CLOSE_ANIMATION_MS = 150;

export function AutomataDetailsModal({
  name,
  isOpen,
  onClose,
  onUpdated,
  onSelectedNameChange,
  onDeleted,
}: {
  name: string;
  isOpen: boolean;
  onClose: () => void;
  onUpdated?: () => void | Promise<void>;
  onSelectedNameChange?: (name: string) => void | Promise<void>;
  onDeleted?: () => void | Promise<void>;
}): React.ReactElement | null {
  const [shouldRender, setShouldRender] = React.useState(isOpen);
  const [isVisible, setIsVisible] = React.useState(false);
  const stableNameRef = React.useRef(name);

  if (name) {
    stableNameRef.current = name;
  }
  const stableName = isOpen || shouldRender ? stableNameRef.current : '';

  React.useEffect(() => {
    if (isOpen) {
      setShouldRender(true);
      requestAnimationFrame(() => {
        requestAnimationFrame(() => setIsVisible(true));
      });
      return;
    }
    setIsVisible(false);
    const timer = setTimeout(() => {
      setShouldRender(false);
    }, CLOSE_ANIMATION_MS);
    return () => clearTimeout(timer);
  }, [isOpen]);

  React.useEffect(() => {
    if (!isOpen) {
      return;
    }
    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }
      if (event.key === 'Escape') {
        onClose();
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const controller = useAutomataDetailController({
    name: stableName,
    enabled: isOpen && !!stableName,
    onUpdated,
    onSelectedNameChange,
    onDeleted,
  });

  if (!shouldRender) {
    return null;
  }

  return (
    <>
      <div
        className="fixed inset-0 z-40 h-screen w-screen bg-black/20"
        onClick={onClose}
      />

      <div
        className={cn(
          'fixed top-0 bottom-0 right-0 z-50 h-screen w-full border-l bg-background transition-all duration-150 ease-out md:w-3/4 xl:w-[56rem]',
          isVisible ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0'
        )}
      >
        <div className="flex h-full min-h-0 flex-col overflow-x-hidden p-4 md:p-6">
          <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
            <AutomataDetailSurface
              key={stableName}
              controller={controller}
              headerCaption="Automata detail"
              renderHeaderActions={(detailController) => (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void detailController.onRename()}
                    disabled={!!detailController.busyAction}
                  >
                    <Pencil className="h-4 w-4" />
                    Rename
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void detailController.onClone()}
                    disabled={!!detailController.busyAction}
                  >
                    <Copy className="h-4 w-4" />
                    Clone
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void detailController.onResetState()}
                    disabled={!!detailController.busyAction}
                  >
                    <RotateCcw className="h-4 w-4" />
                    Reset State
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void detailController.onDelete()}
                    disabled={!!detailController.busyAction}
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete
                  </Button>
                  <Button
                    variant="outline"
                    size="icon-sm"
                    onClick={onClose}
                    title="Close (Esc)"
                    className="relative group"
                  >
                    <X className="h-4 w-4" />
                    <span className="pointer-events-none absolute -top-7 right-0 rounded-sm border bg-muted px-1 text-xs font-medium whitespace-nowrap text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                      Esc
                    </span>
                  </Button>
                </>
              )}
            />
          </div>
        </div>
      </div>
    </>
  );
}
