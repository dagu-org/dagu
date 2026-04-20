import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { History } from 'lucide-react';
import { AutomataSwarmIcon } from '@/components/icons/AutomataSwarmIcon';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { AutomataCockpit } from '@/features/cockpit/components/AutomataCockpit';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';

const COCKPIT_MODE_STORAGE_KEY = 'dagu_cockpit_mode';
type CockpitMode = 'runs' | 'automata';

export default function CockpitPage(): React.ReactElement {
  const { setTitle } = React.useContext(AppBarContext);
  const config = useConfig();
  const [searchParams, setSearchParams] = useSearchParams();
  const automataFeatureEnabled = config.agentEnabled && config.automataEnabled;
  const requestedAutomataName = searchParams.get('automata') || '';
  const requestedMode = searchParams.get('mode');
  const [isTemplateSelectorOpen, setIsTemplateSelectorOpen] = useState(false);
  const [mode, setMode] = useState<CockpitMode>(() => {
    if (requestedMode === 'automata' || requestedAutomataName) {
      return 'automata';
    }
    const stored = localStorage.getItem(COCKPIT_MODE_STORAGE_KEY);
    return stored === 'automata' ? 'automata' : 'runs';
  });
  const { selectedWorkspace, selectedTemplate, selectTemplate } =
    useCockpitState();

  useEffect(() => {
    setTitle('Cockpit');
  }, [setTitle]);

  useEffect(() => {
    if (!automataFeatureEnabled && mode !== 'runs') {
      setMode('runs');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'runs');
    }
  }, [automataFeatureEnabled, mode]);

  useEffect(() => {
    if (
      automataFeatureEnabled &&
      (requestedMode === 'automata' || requestedAutomataName)
    ) {
      setMode('automata');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'automata');
    }
  }, [automataFeatureEnabled, requestedAutomataName, requestedMode]);

  const effectiveMode: CockpitMode =
    automataFeatureEnabled && mode === 'automata' ? 'automata' : 'runs';

  const handleModeChange = (nextMode: CockpitMode) => {
    const resolvedMode = automataFeatureEnabled ? nextMode : 'runs';
    setMode(resolvedMode);
    localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, resolvedMode);
    const nextParams = new URLSearchParams(searchParams);
    if (resolvedMode === 'automata') {
      nextParams.set('mode', 'automata');
    } else {
      nextParams.delete('mode');
      nextParams.delete('automata');
    }
    setSearchParams(nextParams, { replace: true });
    if (resolvedMode !== 'runs') {
      selectTemplate('');
      setIsTemplateSelectorOpen(false);
    }
  };

  const handleAutomataSelectionChange = React.useCallback(
    (name: string | null) => {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set('mode', 'automata');
      if (name) {
        nextParams.set('automata', name);
      } else {
        nextParams.delete('automata');
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
              : 'Monitor Automata lifecycle and workspace activity.'}
          </div>
        </div>
        {automataFeatureEnabled ? (
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
              value="automata"
              groupValue={effectiveMode}
              onClick={() => handleModeChange('automata')}
              aria-label="Automata cockpit"
              className="h-8 px-3"
            >
              <span className="mr-1.5">
                <AutomataSwarmIcon size={16} />
              </span>
              Automata
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
          suspendLoadMore={suspendBackgroundLoading}
        />
      ) : (
        <AutomataCockpit
          selectedWorkspace={selectedWorkspace}
          initialAutomataName={requestedAutomataName}
          onAutomataSelectionChange={handleAutomataSelectionChange}
        />
      )}
    </div>
  );
}
