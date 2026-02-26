import ConfirmModal from '@/ui/ConfirmModal';

type Props = {
  visible: boolean;
  docPath: string;
  onConfirm: () => void;
  onDismiss: () => void;
};

export function DeleteDocDialog({ visible, docPath, onConfirm, onDismiss }: Props) {
  return (
    <ConfirmModal
      title="Delete Document"
      buttonText="Delete"
      visible={visible}
      dismissModal={onDismiss}
      onSubmit={onConfirm}
    >
      <p className="text-sm text-muted-foreground">
        Are you sure you want to delete this document?
      </p>
      <div className="mt-2 px-3 py-1.5 bg-muted rounded-md font-mono text-sm">
        {docPath}
      </div>
    </ConfirmModal>
  );
}
