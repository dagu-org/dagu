import React, { useEffect, useState } from 'react';
import { useCockpitState } from '@/features/cockpit/hooks/useCockpitState';
import { CockpitToolbar } from '@/features/cockpit/components/CockpitToolbar';
import { DateKanbanList } from '@/features/cockpit/components/DateKanbanList';
import { AppBarContext } from '@/contexts/AppBarContext';

export default function CockpitPage(): React.ReactElement {
  const { setTitle } = React.useContext(AppBarContext);
  const [isTemplateSelectorOpen, setIsTemplateSelectorOpen] = useState(false);
  const { selectedWorkspace, selectedTemplate, selectTemplate } =
    useCockpitState();

  useEffect(() => {
    setTitle('Cockpit');
  }, [setTitle]);

  const suspendBackgroundLoading = isTemplateSelectorOpen || !!selectedTemplate;

  return (
    <div className="flex flex-col h-full min-h-0">
      <CockpitToolbar
        selectedWorkspace={selectedWorkspace}
        selectedTemplate={selectedTemplate}
        onSelectTemplate={selectTemplate}
        onTemplateSelectorOpenChange={setIsTemplateSelectorOpen}
      />
      <DateKanbanList
        selectedWorkspace={selectedWorkspace}
        suspendLoadMore={suspendBackgroundLoading}
      />
    </div>
  );
}
