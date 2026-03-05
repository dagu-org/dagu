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
}

export function CockpitToolbar({
  workspaces,
  selectedWorkspace,
  selectedTemplate,
  onSelectWorkspace,
  onCreateWorkspace,
  onDeleteWorkspace,
  onSelectTemplate,
}: Props): React.ReactElement {
  const canWrite = useCanWrite();
  return (
    <div className="flex items-center gap-3 px-3 py-2 border-b border-border flex-wrap">
      <WorkspaceSelector
        workspaces={workspaces}
        selectedWorkspace={selectedWorkspace}
        onSelect={onSelectWorkspace}
        onCreate={onCreateWorkspace}
        onDelete={onDeleteWorkspace}
        canWrite={canWrite}
      />
      <TemplateSelector
        selectedTemplate={selectedTemplate}
        selectedWorkspace={selectedWorkspace}
        onSelect={onSelectTemplate}
      />
      {selectedTemplate && (
        <DAGPreviewModal
          key={selectedTemplate}
          fileName={selectedTemplate}
          selectedWorkspace={selectedWorkspace}
          onClose={() => onSelectTemplate('')}
        />
      )}
    </div>
  );
}
