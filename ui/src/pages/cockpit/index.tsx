import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { History } from 'lucide-react';
import { AutopilotSwarmIcon } from '@/components/icons/AutopilotSwarmIcon';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { AutopilotCockpit } from '@/features/cockpit/components/AutopilotCockpit';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';

const COCKPIT_MODE_STORAGE_KEY = 'dagu_cockpit_mode';
type CockpitMode = 'runs' | 'autopilot';

export default function CockpitPage(): React.ReactElement {
  const { setTitle } = React.useContext(AppBarContext);
  const config = useConfig();
  const [searchParams, setSearchParams] = useSearchParams();
  const autopilotFeatureEnabled = config.agentEnabled && config.autopilotEnabled;
  const legacyRequestedAutopilotName = searchParams.get('automata') || '';
  const requestedAutopilotName =
    searchParams.get('autopilot') || legacyRequestedAutopilotName;
  const requestedMode = searchParams.get('mode');
  const autopilotRequested =
    requestedMode === 'autopilot' ||
    requestedMode === 'automata' ||
    !!requestedAutopilotName;
  const [isTemplateSelectorOpen, setIsTemplateSelectorOpen] = useState(false);
  const [mode, setMode] = useState<CockpitMode>(() => {
    if (autopilotRequested) {
      return 'autopilot';
    }
    const stored = localStorage.getItem(COCKPIT_MODE_STORAGE_KEY);
    return stored === 'autopilot' ? 'autopilot' : 'runs';
  });
  const { selectedWorkspace, workspaceKey, selectedTemplate, selectTemplate } =
    useCockpitState();

  useEffect(() => {
    setTitle('Cockpit');
  }, [setTitle]);

  useEffect(() => {
    if (requestedMode !== 'automata' && !legacyRequestedAutopilotName) {
      return;
    }
    const nextParams = new URLSearchParams(searchParams);
    if (requestedMode === 'automata') {
      nextParams.set('mode', 'autopilot');
    }
    if (legacyRequestedAutopilotName && !searchParams.get('autopilot')) {
      nextParams.set('autopilot', legacyRequestedAutopilotName);
    }
    nextParams.delete('automata');
    setSearchParams(nextParams, { replace: true });
  }, [
    legacyRequestedAutopilotName,
    requestedMode,
    searchParams,
    setSearchParams,
  ]);

  useEffect(() => {
    if (!autopilotFeatureEnabled && mode !== 'runs') {
      setMode('runs');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'runs');
    }
  }, [autopilotFeatureEnabled, mode]);

  useEffect(() => {
    if (autopilotFeatureEnabled && autopilotRequested) {
      setMode('autopilot');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'autopilot');
    }
  }, [autopilotFeatureEnabled, autopilotRequested]);

  const effectiveMode: CockpitMode =
    autopilotFeatureEnabled && mode === 'autopilot' ? 'autopilot' : 'runs';

  const handleModeChange = (nextMode: CockpitMode) => {
    const resolvedMode = autopilotFeatureEnabled ? nextMode : 'runs';
    setMode(resolvedMode);
    localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, resolvedMode);
    const nextParams = new URLSearchParams(searchParams);
    if (resolvedMode === 'autopilot') {
      nextParams.set('mode', 'autopilot');
      nextParams.delete('automata');
    } else {
      nextParams.delete('mode');
      nextParams.delete('autopilot');
      nextParams.delete('automata');
    }
    setSearchParams(nextParams, { replace: true });
    if (resolvedMode !== 'runs') {
      selectTemplate('');
      setIsTemplateSelectorOpen(false);
    }
  };

  const handleAutopilotSelectionChange = React.useCallback(
    (name: string | null) => {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set('mode', 'autopilot');
      nextParams.delete('automata');
      if (name) {
        nextParams.set('autopilot', name);
      } else {
        nextParams.delete('autopilot');
      }
      setSearchParams(nextParams, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  const suspendBackgroundLoading =
    effectiveMode === 'runs' && (isTemplateSelectorOpen || !!selectedTemplate);

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-2xl font-bold text-foreground">Cockpit</div>
          <div className="text-sm text-muted-foreground">
            {effectiveMode === 'runs'
              ? 'Track workspace DAG execution by day.'
              : 'Monitor Autopilot lifecycle and workspace activity.'}
          </div>
        </div>
        {autopilotFeatureEnabled ? (
          <ToggleGroup aria-label="Cockpit mode">
            <ToggleButton
              value="runs"
              groupValue={effectiveMode}
              onClick={() => handleModeChange('runs')}
              aria-label="DAG runs cockpit"
              className="h-8 px-3"
            >
              <History size={16} className="mr-1.5" />
              DAG Runs
            </ToggleButton>
            <ToggleButton
              value="autopilot"
              groupValue={effectiveMode}
              onClick={() => handleModeChange('autopilot')}
              aria-label="Autopilot cockpit"
              className="h-8 px-3"
            >
              <span className="mr-1.5">
                <AutopilotSwarmIcon size={16} />
              </span>
              Autopilot
            </ToggleButton>
          </ToggleGroup>
        ) : null}
      </div>
      <CockpitToolbar
        selectedWorkspace={selectedWorkspace}
        selectedTemplate={selectedTemplate}
        onSelectTemplate={selectTemplate}
        onTemplateSelectorOpenChange={setIsTemplateSelectorOpen}
        showTemplateSelector={effectiveMode === 'runs'}
      />
      {effectiveMode === 'runs' ? (
        <DateKanbanList
          selectedWorkspace={selectedWorkspace}
          workspaceKey={workspaceKey}
          suspendLoadMore={suspendBackgroundLoading}
        />
      ) : (
        <AutopilotCockpit
          selectedWorkspace={selectedWorkspace}
          initialAutopilotName={requestedAutopilotName}
          onAutopilotSelectionChange={handleAutopilotSelectionChange}
        />
      )}
    </div>
  );
}
