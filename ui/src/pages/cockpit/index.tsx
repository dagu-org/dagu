import React, { useEffect } from 'react';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Kanban } from 'lucide-react';

export default function CockpitPage(): React.ReactElement {
  const { setTitle } = React.useContext(AppBarContext);
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

  return (
    <div className="flex flex-col h-full min-h-0">
      <CockpitToolbar
        workspaces={workspaces}
        selectedWorkspace={selectedWorkspace}
        selectedTemplate={selectedTemplate}
        onSelectWorkspace={selectWorkspace}
        onCreateWorkspace={createWorkspace}
        onDeleteWorkspace={deleteWorkspace}
        onSelectTemplate={selectTemplate}
      />
      {selectedWorkspace ? (
        <DateKanbanList selectedWorkspace={selectedWorkspace} />
      ) : (
        <div className="flex flex-col items-center justify-center flex-1 gap-3 text-center text-muted-foreground">
          <Kanban size={40} className="opacity-40" />
          <p className="text-sm">Select or create a workspace to see tasks</p>
        </div>
      )}
    </div>
  );
}
