import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { ModelPresetProvider, type components } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { ModelFormModal } from '../ModelFormModal';

type ModelPreset = components['schemas']['ModelPreset'];

const presets: ModelPreset[] = [
  {
    name: 'OpenAI GPT-5',
    provider: ModelPresetProvider.openai,
    model: 'gpt-5',
    description: 'GPT-5 preset',
  },
  {
    name: 'Claude Sonnet 4.5',
    provider: ModelPresetProvider.anthropic,
    model: 'claude-sonnet-4-5',
  },
];

afterEach(() => {
  cleanup();
});

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
  presets?: ModelPreset[];
};

function renderModal({ open = true, presets: modelPresets = presets }: RenderModalOptions = {}) {
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
          presets={modelPresets}
          onClose={() => undefined}
          onSuccess={() => undefined}
        />
      </AppBarContext.Provider>
    </ConfigContext.Provider>
  );
}

function getDialog() {
  return screen.getByRole('dialog', { name: 'Add Model' });
}

function getDialogQueries() {
  return within(getDialog());
}

async function selectOption(
  user: ReturnType<typeof userEvent.setup>,
  comboboxName: string,
  optionName: string
) {
  await user.click(getDialogQueries().getByRole('combobox', { name: comboboxName }));
  const listbox = await screen.findByRole('listbox');
  await user.click(within(listbox).getByRole('option', { name: optionName }));
  await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument());
}

describe('ModelFormModal', () => {
  it('defaults the create modal provider selection to Local', () => {
    renderModal();
    const dialog = getDialogQueries();

    const providerSelect = dialog.getByRole('combobox', { name: 'Provider' });

    expect(providerSelect).toHaveTextContent('Local');
    expect(dialog.getByLabelText('Model')).toHaveAttribute('placeholder', 'llama3.2:latest');
    expect(dialog.getByText(/Optional for local providers/i)).toBeInTheDocument();
  });

  it('renders Local as the first provider option', async () => {
    const user = userEvent.setup();

    renderModal();
    await user.click(getDialogQueries().getByRole('combobox', { name: 'Provider' }));

    const listbox = await screen.findByRole('listbox');
    const options = within(listbox).getAllByRole('option');

    expect(options.map((option) => option.textContent)).toEqual([
      'Local',
      'Anthropic',
      'OpenAI',
      'Google Gemini',
      'OpenRouter',
      'Z.AI',
    ]);
  });

  it('applies the preset provider and model values when a preset is selected', async () => {
    const user = userEvent.setup();

    renderModal();
    await selectOption(user, 'Import from Preset', 'OpenAI GPT-5');
    const dialog = getDialogQueries();

    expect(dialog.getByRole('combobox', { name: 'Provider' })).toHaveTextContent('OpenAI');
    expect(dialog.getByLabelText('Model')).toHaveValue('gpt-5');
    expect(dialog.queryByText(/Optional for local providers/i)).not.toBeInTheDocument();
  });

  it('resets the create form back to Local after closing and reopening', async () => {
    const user = userEvent.setup();
    const view = renderModal();
    const dialog = getDialogQueries();

    await selectOption(user, 'Provider', 'OpenAI');
    await user.type(dialog.getByLabelText('Model'), 'gpt-5');

    expect(dialog.getByRole('combobox', { name: 'Provider' })).toHaveTextContent('OpenAI');
    expect(dialog.getByLabelText('Model')).toHaveValue('gpt-5');

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
            presets={presets}
            onClose={() => undefined}
            onSuccess={() => undefined}
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
            presets={presets}
            onClose={() => undefined}
            onSuccess={() => undefined}
          />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    );

    const reopenedDialog = getDialogQueries();

    expect(reopenedDialog.getByRole('combobox', { name: 'Provider' })).toHaveTextContent('Local');
    expect(reopenedDialog.getByLabelText('Model')).toHaveValue('');
    expect(reopenedDialog.getByText(/Optional for local providers/i)).toBeInTheDocument();
  });
});
