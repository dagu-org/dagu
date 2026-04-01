// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import * as React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { components } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { ModelFormModal } from '../ModelFormModal';
type MockSelectContextValue = {
  items: Array<{ value: string; label: React.ReactNode }>;
  onValueChange?: (value: string) => void;
  value?: string;
};

const MockSelectContext = React.createContext<MockSelectContextValue>({
  items: [],
});

function MockSelectValue(_props: { placeholder?: string }) {
  return null;
}

function MockSelectContent(_props: { children?: React.ReactNode }) {
  return null;
}

function MockSelectItem(_props: { children?: React.ReactNode; value: string }) {
  return null;
}

function collectSelectItems(children: React.ReactNode): Array<{ value: string; label: React.ReactNode }> {
  const items: Array<{ value: string; label: React.ReactNode }> = [];

  React.Children.forEach(children, (child) => {
    if (!React.isValidElement(child)) {
      return;
    }

    if (child.type === MockSelectItem) {
      items.push({
        value: child.props.value,
        label: child.props.children,
      });
      return;
    }

    if ('children' in child.props) {
      items.push(...collectSelectItems(child.props.children));
    }
  });

  return items;
}

vi.mock('@/components/ui/select', () => {
  const Select = ({
    children,
    onValueChange,
    value,
  }: {
    children?: React.ReactNode;
    onValueChange?: (value: string) => void;
    value?: string;
  }) => {
    const items = collectSelectItems(children);
    return (
      <MockSelectContext.Provider value={{ items, onValueChange, value }}>
        <div>{children}</div>
      </MockSelectContext.Provider>
    );
  };

  const SelectTrigger = React.forwardRef<
    HTMLSelectElement,
    React.ComponentProps<'select'>
  >((props, ref) => {
    const { items, onValueChange, value } = React.useContext(MockSelectContext);
    return (
      <select
        {...props}
        ref={ref}
        value={value ?? ''}
        onChange={(event) => onValueChange?.(event.target.value)}
      >
        <option value="">Select...</option>
        {items.map((item) => (
          <option key={item.value} value={item.value}>
            {item.label}
          </option>
        ))}
      </select>
    );
  });
  SelectTrigger.displayName = 'MockSelectTrigger';

  return {
    Select,
    SelectContent: MockSelectContent,
    SelectItem: MockSelectItem,
    SelectTrigger,
    SelectValue: MockSelectValue,
  };
});

type ModelConfig = components['schemas']['ModelConfigResponse'];
type ModelPreset = components['schemas']['ModelPreset'];

const DISCOVERY_DEBOUNCE_MS = 400;

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
    },
    ...init,
  });
}

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    apiURL: '/api/v1',
    basePath: '/',
    title: 'Dagu',
    navbarColor: '',
    tz: 'UTC',
    tzOffsetInSec: 0,
    version: 'test',
    maxDashboardPageLimit: 100,
    remoteNodes: '',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    agentEnabled: true,
    updateAvailable: false,
    latestVersion: '',
    permissions: {
      writeDags: true,
      runDags: true,
    },
    license: {
      valid: true,
      plan: 'community',
      expiry: '',
      features: [],
      gracePeriod: false,
      community: true,
      source: 'test',
      warningCode: '',
    },
    paths: {
      dagsDir: '',
      logDir: '',
      suspendFlagsDir: '',
      adminLogsDir: '',
      baseConfig: '',
      dagRunsDir: '',
      queueDir: '',
      procDir: '',
      serviceRegistryDir: '',
      configFileUsed: '',
      gitSyncDir: '',
      auditLogsDir: '',
    },
    ...overrides,
  };
}

type RenderModalOptions = {
  open?: boolean;
  model?: ModelConfig;
  presets?: ModelPreset[];
  onClose?: () => void;
  onSuccess?: () => void;
};

function renderModal({
  open = true,
  model,
  presets = [],
  onClose = vi.fn(),
  onSuccess = vi.fn(),
}: RenderModalOptions = {}) {
  return render(
    <ConfigContext.Provider value={makeConfig()}>
      <AppBarContext.Provider
        value={{
          title: '',
          setTitle: () => undefined,
          remoteNodes: ['local'],
          setRemoteNodes: () => undefined,
          selectedRemoteNode: 'local',
          selectRemoteNode: () => undefined,
        }}
      >
        <ModelFormModal
          open={open}
          model={model}
          presets={presets}
          onClose={onClose}
          onSuccess={onSuccess}
        />
      </AppBarContext.Provider>
    </ConfigContext.Provider>
  );
}

async function advanceDiscoveryDebounce() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(DISCOVERY_DEBOUNCE_MS);
  });
}

function selectComboboxOption(
  trigger: HTMLElement,
  optionValue: string
) {
  fireEvent.change(trigger, {
    target: {
      value: optionValue,
    },
  });
}

function chooseLocalProvider() {
  selectComboboxOption(screen.getByLabelText('Provider'), 'local');
}

function choosePreset(presetName: string) {
  const presetWrapper = screen.getByText('Import from Preset').parentElement;
  const presetTrigger = presetWrapper?.querySelector('select');
  if (!presetTrigger) {
    throw new Error('Preset select not found');
  }
  selectComboboxOption(presetTrigger, presetName);
}

function setBaseUrl(url: string) {
  const baseUrlInput = screen.getByLabelText('Base URL (optional)');
  fireEvent.change(baseUrlInput, {
    target: {
      value: url,
    },
  });
}

async function flushAsyncUpdates() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('ModelFormModal', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('fetch', vi.fn());
    vi.clearAllMocks();
    localStorage.clear();
    Element.prototype.hasPointerCapture ??= () => false;
    Element.prototype.setPointerCapture ??= () => undefined;
    Element.prototype.releasePointerCapture ??= () => undefined;
  });

  afterEach(() => {
    cleanup();
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows discovered models and lets the user pick one', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'llama3.2' }, { id: 'gemma3' }],
        warnings: [],
      })
    );

    renderModal();

    chooseLocalProvider();
    expect(screen.getByLabelText('Provider')).toHaveValue('local');
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();

    expect(screen.getByText('Discovered Models')).toBeInTheDocument();

    selectComboboxOption(
      screen.getByLabelText('Discovered Models'),
      'gemma3'
    );

    expect(screen.getByLabelText('Model')).toHaveValue('gemma3');
  });

  it('auto-fills a single discovered model only when the field is blank and untouched', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'llama3.2' }],
        warnings: [],
      })
    );

    renderModal();

    chooseLocalProvider();
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();

    expect(screen.getByLabelText('Model')).toHaveValue('llama3.2');
  });

  it('does not overwrite the existing model in edit mode', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'ollama-auto' }],
        warnings: [],
      })
    );

    renderModal({
      model: {
        id: 'model-1',
        name: 'Existing Local Model',
        provider: 'local',
        model: 'existing-model',
        baseUrl: 'http://localhost:11434',
      },
    });

    await flushAsyncUpdates();
    expect(screen.getByLabelText('Model')).toHaveValue('existing-model');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();

    expect(screen.getByLabelText('Model')).toHaveValue('existing-model');
  });

  it('does not overwrite a model imported from a preset', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'ollama-auto' }],
        warnings: [],
      })
    );

    renderModal({
      presets: [
        {
          name: 'Local Preset',
          provider: 'local',
          model: 'preset-model',
        },
      ],
    });

    choosePreset('Local Preset');
    expect(screen.getByLabelText('Model')).toHaveValue('preset-model');
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();

    expect(screen.getByLabelText('Model')).toHaveValue('preset-model');
  });

  it('preserves manual model overrides across later discoveries', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'ollama-auto' }],
        warnings: [],
      })
    );

    renderModal();

    chooseLocalProvider();
    const modelInput = screen.getByLabelText('Model');
    fireEvent.change(modelInput, {
      target: {
        value: 'manual-model',
      },
    });
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();

    expect(modelInput).toHaveValue('manual-model');
  });

  it('ignores stale discovery responses when the base url changes quickly', async () => {
    const resolvers: Array<(response: Response) => void> = [];

    vi.mocked(fetch).mockImplementation(
      () =>
        new Promise<Response>((resolve) => {
          resolvers.push(resolve);
        })
    );

    renderModal();

    chooseLocalProvider();
    expect(screen.getByLabelText('Provider')).toHaveValue('local');
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();
    expect(fetch).toHaveBeenCalledTimes(1);

    setBaseUrl('http://localhost:22434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();
    expect(fetch).toHaveBeenCalledTimes(2);

    resolvers[1]?.(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'second-model' }],
        warnings: [],
      })
    );

    await flushAsyncUpdates();
    expect(screen.getByLabelText('Model')).toHaveValue('second-model');

    resolvers[0]?.(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'first-model' }],
        warnings: [],
      })
    );

    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.getByLabelText('Model')).toHaveValue('second-model');
  });

  it('clears discovery state when switching away from the local provider', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({
        success: true,
        supported: true,
        models: [{ id: 'llama3.2' }, { id: 'gemma3' }],
        warnings: [],
      })
    );

    renderModal();

    chooseLocalProvider();
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();
    expect(screen.getByText('Discovered Models')).toBeInTheDocument();

    selectComboboxOption(screen.getByLabelText('Provider'), 'openai');
    await flushAsyncUpdates();

    expect(screen.queryByText('Discovered Models')).not.toBeInTheDocument();
    expect(screen.queryByText(/No models were discovered/i)).not.toBeInTheDocument();
  });

  it('resets discovery models and errors when the modal closes and reopens', async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        success: false,
        supported: true,
        models: [],
        warnings: [],
        error: 'Discovery failed',
      })
    );

    const view = renderModal();

    chooseLocalProvider();
    setBaseUrl('http://localhost:11434');
    await advanceDiscoveryDebounce();
    await flushAsyncUpdates();
    expect(screen.getByText('Discovery failed')).toBeInTheDocument();

    view.rerender(
      <ConfigContext.Provider value={makeConfig()}>
        <AppBarContext.Provider
          value={{
            title: '',
            setTitle: () => undefined,
            remoteNodes: ['local'],
            setRemoteNodes: () => undefined,
            selectedRemoteNode: 'local',
            selectRemoteNode: () => undefined,
          }}
        >
          <ModelFormModal
            open={false}
            presets={[]}
            onClose={vi.fn()}
            onSuccess={vi.fn()}
          />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    );

    view.rerender(
      <ConfigContext.Provider value={makeConfig()}>
        <AppBarContext.Provider
          value={{
            title: '',
            setTitle: () => undefined,
            remoteNodes: ['local'],
            setRemoteNodes: () => undefined,
            selectedRemoteNode: 'local',
            selectRemoteNode: () => undefined,
          }}
        >
          <ModelFormModal
            open={true}
            presets={[]}
            onClose={vi.fn()}
            onSuccess={vi.fn()}
          />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    );
    await flushAsyncUpdates();

    expect(screen.queryByText('Discovery failed')).not.toBeInTheDocument();
    expect(screen.queryByText('Discovered Models')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Base URL (optional)')).toHaveValue('');
  });
});
