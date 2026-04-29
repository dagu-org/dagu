import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { History } from 'lucide-react';
import { ControllerSwarmIcon } from '@/components/icons/ControllerSwarmIcon';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { ControllerCockpit } from '@/features/cockpit/components/ControllerCockpit';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';

const COCKPIT_MODE_STORAGE_KEY = 'dagu_cockpit_mode';
type CockpitMode = 'runs' | 'controller';

export default function CockpitPage(): React.ReactElement {
  const { setTitle } = React.useContext(AppBarContext);
  const config = useConfig();
  const [searchParams, setSearchParams] = useSearchParams();
  const controllerFeatureEnabled = config.agentEnabled;
  const requestedControllerName = searchParams.get('controller') || '';
  const requestedMode = searchParams.get('mode');
  const controllerRequested =
    requestedMode === 'controller' || !!requestedControllerName;
  const [isTemplateSelectorOpen, setIsTemplateSelectorOpen] = useState(false);
  const [mode, setMode] = useState<CockpitMode>(() => {
    if (controllerRequested) {
      return 'controller';
    }
    const stored = localStorage.getItem(COCKPIT_MODE_STORAGE_KEY);
    return stored === 'controller' ? 'controller' : 'runs';
  });
  const { selectedWorkspace, workspaceKey, selectedTemplate, selectTemplate } =
    useCockpitState();

  useEffect(() => {
    setTitle('Cockpit');
  }, [setTitle]);

  useEffect(() => {
    if (!controllerFeatureEnabled && mode !== 'runs') {
      setMode('runs');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'runs');
    }
  }, [controllerFeatureEnabled, mode]);

  useEffect(() => {
    if (controllerFeatureEnabled && controllerRequested) {
      setMode('controller');
      localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, 'controller');
    }
  }, [controllerFeatureEnabled, controllerRequested]);

  const effectiveMode: CockpitMode =
    controllerFeatureEnabled && mode === 'controller' ? 'controller' : 'runs';

  const handleModeChange = (nextMode: CockpitMode) => {
    const resolvedMode = controllerFeatureEnabled ? nextMode : 'runs';
    setMode(resolvedMode);
    localStorage.setItem(COCKPIT_MODE_STORAGE_KEY, resolvedMode);
    const nextParams = new URLSearchParams(searchParams);
    if (resolvedMode === 'controller') {
      nextParams.set('mode', 'controller');
    } else {
      nextParams.delete('mode');
      nextParams.delete('controller');
    }
    setSearchParams(nextParams, { replace: true });
    if (resolvedMode !== 'runs') {
      selectTemplate('');
      setIsTemplateSelectorOpen(false);
    }
  };

  const handleControllerSelectionChange = React.useCallback(
    (name: string | null) => {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.set('mode', 'controller');
      if (name) {
        nextParams.set('controller', name);
      } else {
        nextParams.delete('controller');
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
              : 'Monitor Controller lifecycle and workspace activity.'}
          </div>
        </div>
        {controllerFeatureEnabled ? (
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
              value="controller"
              groupValue={effectiveMode}
              onClick={() => handleModeChange('controller')}
              aria-label="Controller cockpit"
              className="h-8 px-3"
            >
              <span className="mr-1.5">
                <ControllerSwarmIcon size={16} />
              </span>
              Controller
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
        <ControllerCockpit
          selectedWorkspace={selectedWorkspace}
          initialControllerName={requestedControllerName}
          onControllerSelectionChange={handleControllerSelectionChange}
        />
      )}
    </div>
  );
}
