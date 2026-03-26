import React from 'react';
import { WorkspaceSelector } from './WorkspaceSelector';
import { TemplateSelector } from './TemplateSelector';
import { DAGPreviewModal } from './DAGPreviewModal';
import { useCanWrite } from '@/contexts/AuthContext';
import type { components } from '@/api/v1/schema';

type WorkspaceResponse = components['schemas']['WorkspaceResponse'];

interface Props {
  workspaces: WorkspaceResponse[];
  selectedWorkspace: string;
  selectedTemplate: string;
  onSelectWorkspace: (name: string) => void;
  onCreateWorkspace: (name: string) => void;
  onDeleteWorkspace: (id: string) => void;
  onSelectTemplate: (fileName: string) => void;
  onTemplateSelectorOpenChange?: (isOpen: boolean) => void;
  showTemplateSelector?: boolean;
}

export function CockpitToolbar({
  workspaces,
  selectedWorkspace,
  selectedTemplate,
  onSelectWorkspace,
  onCreateWorkspace,
  onDeleteWorkspace,
  onSelectTemplate,
  onTemplateSelectorOpenChange,
  showTemplateSelector = true,
}: Props): React.ReactElement {
  const canWrite = useCanWrite();
  return (
    <div className="flex items-center gap-3 flex-wrap mb-2 max-md:flex-col max-md:items-stretch">
      <WorkspaceSelector
        workspaces={workspaces}
        selectedWorkspace={selectedWorkspace}
        onSelect={onSelectWorkspace}
        onCreate={onCreateWorkspace}
        onDelete={onDeleteWorkspace}
        canWrite={canWrite}
      />
      {showTemplateSelector ? (
        <>
          <TemplateSelector
            selectedTemplate={selectedTemplate}
            selectedWorkspace={selectedWorkspace}
            onSelect={onSelectTemplate}
            onOpenChange={onTemplateSelectorOpenChange}
          />
          <DAGPreviewModal
            fileName={selectedTemplate}
            isOpen={!!selectedTemplate}
            selectedWorkspace={selectedWorkspace}
            onClose={() => onSelectTemplate('')}
          />
        </>
      ) : null}
    </div>
  );
}
