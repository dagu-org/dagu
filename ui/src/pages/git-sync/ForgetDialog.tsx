import ConfirmModal from '@/ui/ConfirmModal';

interface ForgetDialogProps {
  open: boolean;
  itemId: string;
  isForgetting: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ForgetDialog({
  open,
  itemId,
  isForgetting,
  onConfirm,
  onCancel,
}: ForgetDialogProps) {
  return (
    <ConfirmModal
      title="Forget Item"
      buttonText={isForgetting ? 'Forgetting...' : 'Forget'}
      visible={open}
      dismissModal={onCancel}
      onSubmit={onConfirm}
    >
      <p className="text-sm text-muted-foreground">
        Remove{' '}
        <span className="font-mono font-medium text-foreground">{itemId}</span>{' '}
        from sync tracking? This does not delete the file from the remote
        repository.
      </p>
    </ConfirmModal>
  );
}
