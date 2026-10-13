// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import * as React from 'react';

// The dialog loads on first open so the shell does not pay for it upfront.
const CommandPaletteDialog = React.lazy(() => import('./CommandPaletteDialog'));

// Editors and terminals keep their own Cmd/Ctrl+K chords.
const KEYBOARD_OWNER_SELECTOR = '.monaco-editor, .xterm, .cm-editor';

type CommandPaletteContextValue = { open: () => void };

const CommandPaletteContext = React.createContext<CommandPaletteContextValue>({
  open: () => undefined,
});

/** Opens the command palette from any control inside its provider. */
export function useCommandPalette(): CommandPaletteContextValue {
  return React.useContext(CommandPaletteContext);
}

/** Display text for the palette shortcut on the current platform. */
export function commandPaletteShortcut(): string {
  const platform =
    typeof navigator === 'undefined' ? '' : navigator.platform || '';
  return /mac|iphone|ipad/i.test(platform) ? '⌘K' : 'Ctrl K';
}

/**
 * Provides the command palette and its Cmd/Ctrl+K shortcut. Must render
 * inside the router.
 */
export function CommandPaletteProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const [isOpen, setIsOpen] = React.useState(false);
  const [hasOpened, setHasOpened] = React.useState(false);
  const open = React.useCallback(() => {
    setHasOpened(true);
    setIsOpen(true);
  }, []);

  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (
        event.key.toLowerCase() !== 'k' ||
        !(event.metaKey || event.ctrlKey) ||
        event.altKey ||
        event.shiftKey
      ) {
        return;
      }
      const target = event.target;
      if (
        target instanceof Element &&
        target.closest(KEYBOARD_OWNER_SELECTOR)
      ) {
        return;
      }
      event.preventDefault();
      setHasOpened(true);
      setIsOpen((current) => !current);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const value = React.useMemo(() => ({ open }), [open]);

  return (
    <CommandPaletteContext.Provider value={value}>
      {children}
      {hasOpened && (
        <React.Suspense fallback={null}>
          <CommandPaletteDialog open={isOpen} onOpenChange={setIsOpen} />
        </React.Suspense>
      )}
    </CommandPaletteContext.Provider>
  );
}
