import React from 'react';
import { TemplateSelector } from './TemplateSelector';
import { DAGPreviewModal } from './DAGPreviewModal';

interface Props {
  selectedWorkspace: string;
  selectedTemplate: string;
  onSelectTemplate: (fileName: string) => void;
  onTemplateSelectorOpenChange?: (isOpen: boolean) => void;
  showTemplateSelector?: boolean;
}

export function CockpitToolbar({
  selectedWorkspace,
  selectedTemplate,
  onSelectTemplate,
  onTemplateSelectorOpenChange,
  showTemplateSelector = true,
}: Props): React.ReactElement {
  return (
    <div className="flex items-center gap-3 flex-wrap mb-2 max-md:flex-col max-md:items-stretch">
      {showTemplateSelector ? (
        <>
          <TemplateSelector
            selectedTemplate={selectedTemplate}
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
