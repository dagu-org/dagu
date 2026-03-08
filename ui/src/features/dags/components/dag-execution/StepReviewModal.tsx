import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Ban, Check, RotateCcw, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components } from '../../../../api/v1/schema';

type Step = components['schemas']['Step'];
type NodeData = components['schemas']['Node'];
type DAGRunDetails = components['schemas']['DAGRunDetails'];

type WaitConfig = {
  prompt?: string;
  input?: string[];
  required?: string[];
};

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step: Step;
  node: NodeData;
  dagName: string;
  dagRunId: string;
  dagRun: DAGRunDetails;
  onApprove?: (inputs: Record<string, string>) => Promise<void>;
  onPushBack?: (inputs: Record<string, string>) => Promise<void>;
  onReject?: (reason: string) => Promise<void>;
  compact?: boolean;
};

export function StepReviewModal({
  visible,
  dismissModal,
  step,
  node,
  onApprove,
  onPushBack,
  onReject,
}: Props) {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [rejectionReason, setRejectionReason] = useState('');

  const approvalConfig = step.approval;
  const waitConfig: WaitConfig = approvalConfig
    ? { prompt: approvalConfig.prompt, input: approvalConfig.input, required: approvalConfig.required }
    : (step.executorConfig?.config as WaitConfig) || {};
  const inputFields = waitConfig.input || [];
  const requiredFields = waitConfig.required || [];

  const isValid = requiredFields.every(
    (field) => inputs[field] && inputs[field].trim() !== ''
  );

  useEffect(() => {
    if (visible) {
      setInputs({});
      setError(null);
      setLoading(false);
      setRejectionReason('');
    }
  }, [visible]);

  const handleAction = async (
    action: (arg: Record<string, string> | string) => Promise<void>,
    arg: Record<string, string> | string,
    errorLabel: string
  ) => {
    setLoading(true);
    setError(null);
    try {
      await action(arg);
      dismissModal();
    } catch (e) {
      const err = e as { message?: string };
      setError(err.message || `Failed to ${errorLabel} step`);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = () => {
    if (onReject) {
      handleAction(onReject as (arg: Record<string, string> | string) => Promise<void>, rejectionReason, 'reject');
    } else if (onApprove) {
      if (!isValid) {
        setError('Please fill in all required fields');
        return;
      }
      handleAction(onApprove as (arg: Record<string, string> | string) => Promise<void>, inputs, 'approve');
    } else if (onPushBack) {
      if (!isValid) {
        setError('Please fill in all required fields');
        return;
      }
      handleAction(onPushBack as (arg: Record<string, string> | string) => Promise<void>, inputs, 'retry');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey && !loading) {
      e.preventDefault();
      handleSubmit();
    }
  };

  // Determine action label and styling
  const actionLabel = onReject ? 'Reject' : onApprove ? 'Approve' : 'Retry';
  const actionIcon = onReject
    ? <Ban className="h-4 w-4" />
    : onApprove
      ? <Check className="h-4 w-4" />
      : <RotateCcw className="h-4 w-4" />;
  const actionVariant = onReject ? 'destructive' as const : onApprove ? 'primary' as const : 'outline' as const;

  return (
    <Dialog open={visible} onOpenChange={dismissModal}>
      <DialogContent className="sm:max-w-[450px]">
        <DialogHeader>
          <DialogTitle className="text-base">
            {actionLabel}: {step.name}
          </DialogTitle>
        </DialogHeader>

        <div className="py-2 space-y-3" onKeyDown={handleKeyDown}>
          {/* Input fields for approve/retry */}
          {!onReject && inputFields.length > 0 && (
            <div className="space-y-3">
              {inputFields.map((field) => {
                const isRequired = requiredFields.includes(field);
                const fieldId = `review-input-${field}`;
                return (
                  <div key={field}>
                    <label htmlFor={fieldId} className="block text-sm font-medium mb-1">
                      {field}
                      {isRequired && <span className="text-error ml-1">*</span>}
                    </label>
                    <input
                      id={fieldId}
                      type="text"
                      value={inputs[field] || ''}
                      onChange={(e) =>
                        setInputs((prev) => ({ ...prev, [field]: e.target.value }))
                      }
                      className="w-full px-3 py-1 text-sm border border-border rounded bg-background focus:outline-none focus:border-ring"
                      placeholder={`Enter ${field}`}
                    />
                  </div>
                );
              })}
            </div>
          )}

          {/* Rejection reason */}
          {onReject && (
            <textarea
              className="w-full px-3 py-2 text-sm border border-border rounded bg-background focus:outline-none focus:border-ring resize-none"
              placeholder="Reason (optional)..."
              rows={2}
              value={rejectionReason}
              onChange={(e) => setRejectionReason(e.target.value)}
            />
          )}

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
            variant={onReject ? 'default' : actionVariant}
            onClick={handleSubmit}
            disabled={loading || (!onReject && !isValid && requiredFields.length > 0)}
          >
            {actionIcon}
            {loading ? `${actionLabel}...` : actionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
