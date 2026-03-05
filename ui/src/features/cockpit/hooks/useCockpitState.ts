import { useCallback, useSyncExternalStore } from 'react';

const STORAGE_KEY = 'dagu-cockpit-state';

interface CockpitState {
  workspaces: string[];
  selectedWorkspace: string;
  selectedTemplate: string;
}

const defaultState: CockpitState = {
  workspaces: [],
  selectedWorkspace: '',
  selectedTemplate: '',
};

// Cached snapshot — only replaced when the serialized value changes.
let cachedRaw: string | null = null;
let cachedState: CockpitState = defaultState;

function readState(): CockpitState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw !== cachedRaw) {
      cachedRaw = raw;
      cachedState = raw ? { ...defaultState, ...JSON.parse(raw) } : defaultState;
    }
  } catch { /* ignore */ }
  return cachedState;
}

function writeState(state: CockpitState): void {
  try {
    const raw = JSON.stringify(state);
    localStorage.setItem(STORAGE_KEY, raw);
    cachedRaw = raw;
    cachedState = state;
  } catch { /* ignore */ }
  notify();
}

// Simple pub/sub for useSyncExternalStore
let listeners: Array<() => void> = [];
function subscribe(listener: () => void): () => void {
  listeners = [...listeners, listener];
  return () => { listeners = listeners.filter((l) => l !== listener); };
}
function notify(): void {
  for (const l of listeners) l();
}
function getSnapshot(): CockpitState {
  return readState();
}

export function useCockpitState() {
  const state = useSyncExternalStore(subscribe, getSnapshot);

  const createWorkspace = useCallback((name: string) => {
    const s = readState();
    if (!name || s.workspaces.includes(name)) return;
    writeState({ ...s, workspaces: [...s.workspaces, name], selectedWorkspace: name });
  }, []);

  const deleteWorkspace = useCallback((name: string) => {
    const s = readState();
    const workspaces = s.workspaces.filter((w) => w !== name);
    writeState({
      ...s,
      workspaces,
      selectedWorkspace: s.selectedWorkspace === name ? '' : s.selectedWorkspace,
    });
  }, []);

  const selectWorkspace = useCallback((name: string) => {
    const s = readState();
    writeState({ ...s, selectedWorkspace: name });
  }, []);

  const selectTemplate = useCallback((fileName: string) => {
    const s = readState();
    writeState({ ...s, selectedTemplate: fileName });
  }, []);

  return {
    ...state,
    createWorkspace,
    deleteWorkspace,
    selectWorkspace,
    selectTemplate,
  };
}
