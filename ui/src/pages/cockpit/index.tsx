import React, { useEffect } from 'react';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { AppBarContext } from '@/contexts/AppBarContext';

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
      <DateKanbanList
        selectedWorkspace={selectedWorkspace}
        suspendLoadMore={!!selectedTemplate}
      />
    </div>
  );
}
