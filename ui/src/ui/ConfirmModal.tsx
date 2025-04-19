import React, { useEffect } from 'react';
import { Button } from '@/components/ui/button';

type Props = {
  title: string;
  buttonText: string;
  children: React.ReactNode;
  visible: boolean;
  dismissModal: () => void;
  onSubmit: () => void;
};

function ConfirmModal({
  children,
  title,
  buttonText,
  visible,
  dismissModal,
  onSubmit,
}: Props) {
  useEffect(() => {
    const callback = (event: KeyboardEvent) => {
      const e = event || window.event;
      if (e.key == 'Escape' || e.key == 'Esc') {
        dismissModal();
      }
    };
    document.addEventListener('keydown', callback);
    return () => {
      document.removeEventListener('keydown', callback);
    };
  }, [dismissModal]);

  if (!visible) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg border-2 border-black bg-white p-6 shadow-xl">
        <div className="flex items-center justify-center">
          <h2 className="text-xl font-semibold">{title}</h2>
        </div>

        <div className="mt-4 flex flex-col space-y-4">
          <div>{children}</div>

          <Button
            variant="outline"
            className="cursor-pointer"
            onClick={() => onSubmit()}
          >
            {buttonText}
          </Button>

          <Button
            variant="outline"
            className="text-destructive hover:bg-destructive/10 cursor-pointer"
            onClick={dismissModal}
          >
            Cancel
          </Button>
        </div>
      </div>
    </div>
  );
}

export default ConfirmModal;
