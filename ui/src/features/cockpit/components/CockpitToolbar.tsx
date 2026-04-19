import React from 'react';
import { TemplateSelector } from './TemplateSelector';
import { DAGPreviewModal } from './DAGPreviewModal';

interface Props {
  selectedWorkspace: string;
  selectedTemplate: string;
  onSelectTemplate: (fileName: string) => void;
  onTemplateSelectorOpenChange?: (isOpen: boolean) => void;
}

export function CockpitToolbar({
  selectedWorkspace,
  selectedTemplate,
  onSelectTemplate,
  onTemplateSelectorOpenChange,
}: Props): React.ReactElement {
  return (
    <div className="flex items-center gap-3 flex-wrap mb-2 max-md:flex-col max-md:items-stretch">
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
    </div>
  );
}
