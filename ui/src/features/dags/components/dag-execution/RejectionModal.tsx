import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Ban, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components } from '../../../../api/v1/schema';

type Step = components['schemas']['Step'];

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step: Step;
  onReject: (reason: string) => Promise<void>;
};

/**
 * RejectionModal displays a form for rejecting a wait step with an optional reason.
 */
export function RejectionModal({ visible, dismissModal, step, onReject }: Props) {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset state when modal opens
  useEffect(() => {
    if (visible) {
      setReason('');
      setError(null);
      setLoading(false);
    }
  }, [visible]);

  const handleReject = async () => {
    setLoading(true);
    setError(null);

    try {
      await onReject(reason);
      dismissModal();
    } catch (e) {
      const err = e as { message?: string };
      setError(err.message || 'Failed to reject step');
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey && !loading) {
      e.preventDefault();
      handleReject();
    }
  };

  return (
    <Dialog open={visible} onOpenChange={dismissModal}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="text-base">
            Reject: {step.name}
          </DialogTitle>
        </DialogHeader>

        <div className="py-2 space-y-4">
          <div className="text-sm text-muted-foreground">
            Are you sure you want to reject this step? Dependent steps will be aborted.
          </div>

          {/* Optional reason field */}
          <div>
            <label htmlFor="rejection-reason" className="block text-sm font-semibold mb-1">
              Reason <span className="text-muted-foreground">(optional)</span>
            </label>
            <textarea
              id="rejection-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              onKeyDown={handleKeyDown}
              className="w-full px-3 py-2 text-sm border border-border rounded bg-background focus:outline-none focus:border-ring resize-none"
              placeholder="Enter a reason for rejection..."
              rows={3}
            />
          </div>

          {/* Error message */}
          {error && (
            <div className="text-sm text-error bg-error-muted p-2 rounded">
              {error}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            size="sm"
            variant="ghost"
            onClick={dismissModal}
            disabled={loading}
          >
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={handleReject}
            disabled={loading}
            className="bg-error hover:bg-error/90"
          >
            <Ban className="h-4 w-4" />
            {loading ? 'Rejecting...' : 'Reject'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
