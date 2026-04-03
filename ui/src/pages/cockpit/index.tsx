import React, { useEffect, useState } from 'react';
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
  const [isTemplateSelectorOpen, setIsTemplateSelectorOpen] = useState(false);
  const [mode, setMode] = useState<CockpitMode>(() => {
    const stored = localStorage.getItem(COCKPIT_MODE_STORAGE_KEY);
    return stored === 'automata' ? 'automata' : 'runs';
  });
  const {
    workspaces,
    selectedWorkspace,
    selectedTemplate,
    createWorkspace,
    deleteWorkspace,
    selectWorkspace,
    selectTemplate,
  } = useCockpitState();

  useEffect(() => {
    setTitle('Cockpit');
  }, [setTitle]);

  useEffect(() => {
    if (!config.agentEnabled && mode !== 'runs') {
      setMode('runs');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'runs');
    }
  }, [config.agentEnabled, mode]);

  const effectiveMode: CockpitMode =
    config.agentEnabled && mode === 'automata' ? 'automata' : 'runs';

  const handleModeChange = (nextMode: CockpitMode) => {
    const resolvedMode = config.agentEnabled ? nextMode : 'runs';
    setMode(resolvedMode);
    localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, resolvedMode);
    if (resolvedMode !== 'runs') {
      selectTemplate('');
      setIsTemplateSelectorOpen(false);
    }
  };

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
        {config.agentEnabled ? (
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
              <span className="mr-1.5"><AutomataSwarmIcon size={16} /></span>
              Automata
            </ToggleButton>
          </ToggleGroup>
        ) : null}
      </div>
      <CockpitToolbar
        workspaces={workspaces}
        selectedWorkspace={selectedWorkspace}
        selectedTemplate={selectedTemplate}
        onSelectWorkspace={selectWorkspace}
        onCreateWorkspace={createWorkspace}
        onDeleteWorkspace={deleteWorkspace}
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
        <AutomataCockpit selectedWorkspace={selectedWorkspace} />
      )}
    </div>
  );
}
