import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { ArrowLeft, Ban, Check, RotateCcw, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components, Stream } from '../../../../api/v1/schema';
import { InlineLogViewer } from '../common/InlineLogViewer';

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
  /** Hide step output and prompt (when already shown in parent context) */
  compact?: boolean;
};

export function StepReviewModal({
  visible,
  dismissModal,
  step,
  node,
  dagName,
  dagRunId,
  dagRun,
  onApprove,
  onPushBack,
  onReject,
  compact,
}: Props) {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [rejectionReason, setRejectionReason] = useState('');

  // Extract config from step.approval first, fall back to executorConfig for legacy hitl
  const approvalConfig = step.approval;
  const waitConfig: WaitConfig = approvalConfig
    ? { prompt: approvalConfig.prompt, input: approvalConfig.input, required: approvalConfig.required }
    : (step.executorConfig?.config as WaitConfig) || {};
  const inputFields = waitConfig.input || [];
  const requiredFields = waitConfig.required || [];
  const prompt = waitConfig.prompt || step.description;
  const iteration = node?.approvalIteration || 0;

  // Check if all required fields are filled
  const isValid = requiredFields.every(
    (field) => inputs[field] && inputs[field].trim() !== ''
  );

  // Reset state when modal opens
  useEffect(() => {
    if (visible) {
      setInputs({});
      setError(null);
      setLoading(false);
      setRejecting(false);
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

  const handleApprove = () => {
    if (!isValid) {
      setError('Please fill in all required fields');
      return;
    }
    handleAction(onApprove as (arg: Record<string, string> | string) => Promise<void>, inputs, 'approve');
  };

  const handlePushBack = () => {
    if (!isValid || !onPushBack) return;
    handleAction(onPushBack as (arg: Record<string, string> | string) => Promise<void>, inputs, 'push back');
  };

  const handleConfirmReject = () => {
    handleAction(onReject as (arg: Record<string, string> | string) => Promise<void>, rejectionReason, 'reject');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey && !loading) {
      e.preventDefault();
      if (rejecting) {
        handleConfirmReject();
      } else if (isValid) {
        handleApprove();
      }
    }
  };

  return (
    <Dialog open={visible} onOpenChange={dismissModal}>
      <DialogContent className="sm:max-w-[700px]">
        <DialogHeader>
          <DialogTitle className="text-base flex items-center gap-2">
            <span>{step.name}</span>
            {iteration > 0 && (
              <span className="text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                Iteration {iteration}
              </span>
            )}
          </DialogTitle>
        </DialogHeader>

        <div className="py-2 space-y-4" onKeyDown={handleKeyDown}>
          {/* Step Output Panel */}
          {!compact && (
            <div>
              <div className="text-xs font-medium text-muted-foreground mb-1">Step Output</div>
              <div className="max-h-[250px] overflow-y-auto rounded border border-border">
                <InlineLogViewer
                  dagName={dagName}
                  dagRunId={dagRunId}
                  stepName={step.name}
                  stream={Stream.stdout}
                  dagRun={dagRun}
                />
              </div>
            </div>
          )}

          {/* Prompt message */}
          {prompt && (
            <div className="text-base whitespace-pre-wrap">
              {prompt}
            </div>
          )}

          {/* Input fields */}
          {inputFields.length > 0 && (
            <div className="space-y-3">
              {inputFields.map((field) => {
                const isRequired = requiredFields.includes(field);
                const fieldId = `review-input-${field}`;
                return (
                  <div key={field}>
                    <label htmlFor={fieldId} className="block text-sm font-medium mb-1">
                      {field}
                      {isRequired && (
                        <span className="text-error ml-1">*</span>
                      )}
                    </label>
                    <input
                      id={fieldId}
                      type="text"
                      value={inputs[field] || ''}
                      onChange={(e) =>
                        setInputs((prev) => ({
                          ...prev,
                          [field]: e.target.value,
                        }))
                      }
                      className="w-full px-3 py-1 text-sm border border-border rounded bg-background focus:outline-none focus:border-ring"
                      placeholder={`Enter ${field}`}
                    />
                  </div>
                );
              })}
            </div>
          )}

          {/* Rejection confirmation */}
          {rejecting && (
            <div className="space-y-2 border-t border-border pt-3">
              <div className="text-xs font-medium text-error">
                Are you sure? Dependent steps will be aborted.
              </div>
              <textarea
                className="w-full px-3 py-2 text-sm border border-error/30 rounded bg-background focus:outline-none focus:border-error resize-none"
                placeholder="Reason for rejection (optional)..."
                rows={2}
                value={rejectionReason}
                onChange={(e) => setRejectionReason(e.target.value)}
              />
            </div>
          )}

          {/* Error message */}
          {error && (
            <div className="text-sm text-error bg-error-muted p-2 rounded">
              {error}
            </div>
          )}
        </div>

        <DialogFooter>
          {!rejecting ? (
            <>
              <Button
                size="sm"
                variant="ghost"
                onClick={dismissModal}
                disabled={loading}
              >
                <X className="h-4 w-4" />
                Cancel
              </Button>
              {onPushBack && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handlePushBack}
                  disabled={loading || (!isValid && requiredFields.length > 0)}
                >
                  <RotateCcw className="h-4 w-4" />
                  {loading ? 'Retrying...' : 'Retry'}
                </Button>
              )}
              {onReject && (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => setRejecting(true)}
                  disabled={loading}
                >
                  <Ban className="h-4 w-4" />
                  Reject
                </Button>
              )}
              {onApprove && (
                <Button
                  size="sm"
                  variant="primary"
                  onClick={handleApprove}
                  disabled={loading || (!isValid && requiredFields.length > 0)}
                >
                  <Check className="h-4 w-4" />
                  {loading ? 'Approving...' : 'Approve'}
                </Button>
              )}
            </>
          ) : (
            <>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setRejecting(false)}
                disabled={loading}
              >
                <ArrowLeft className="h-4 w-4" />
                Back
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={handleConfirmReject}
                disabled={loading}
              >
                <Ban className="h-4 w-4" />
                {loading ? 'Rejecting...' : 'Confirm Reject'}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
