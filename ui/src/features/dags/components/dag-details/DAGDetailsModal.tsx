import React from 'react';
import DAGDetailsSidePanel from './DAGDetailsSidePanel';

type Props = {
  fileName: string;
  isOpen: boolean;
  onClose: () => void;
};

function DAGDetailsModal({
  fileName,
  isOpen,
  onClose,
}: Props): React.ReactElement | null {
  return (
    <DAGDetailsSidePanel
      fileName={fileName}
      isOpen={isOpen}
      onClose={onClose}
      initialTab="status"
      toolbarHint={
        <>
          Use{' '}
          <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">↑</kbd>{' '}
          <kbd className="px-1 py-0.5 bg-muted rounded text-xs font-mono">↓</kbd>{' '}
          to navigate DAGs
        </>
      }
    />
  );
}

export default DAGDetailsModal;
