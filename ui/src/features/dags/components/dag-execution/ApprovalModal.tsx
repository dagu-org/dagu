import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Check, RotateCcw, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components } from '../../../../api/v1/schema';

type Step = components['schemas']['Step'];
type NodeData = components['schemas']['Node'];

type WaitConfig = {
  prompt?: string;
  input?: string[];
  required?: string[];
};

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step: Step;
  node?: NodeData;
  onApprove: (inputs: Record<string, string>) => Promise<void>;
  onPushBack?: (inputs: Record<string, string>) => Promise<void>;
};

/**
 * ApprovalModal displays a form for approving or pushing back a step with optional input fields.
 */
export function ApprovalModal({ visible, dismissModal, step, node, onApprove, onPushBack }: Props) {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Extract config from step.approval first, fall back to executorConfig for legacy hitl
  const approvalConfig = step.approval;
  const waitConfig: WaitConfig = approvalConfig
    ? { prompt: approvalConfig.prompt, input: approvalConfig.input, required: approvalConfig.required }
    : (step.executorConfig?.config as WaitConfig) || {};
  const inputFields = waitConfig.input || [];
  const requiredFields = waitConfig.required || [];
  const prompt = waitConfig.prompt || step.description;
  const iteration = node?.approvalIteration || 0;

  // Reset state when modal opens
  useEffect(() => {
    if (visible) {
      setInputs({});
      setError(null);
      setLoading(false);
    }
  }, [visible]);

  // Check if all required fields are filled
  const isValid = requiredFields.every(
    (field) => inputs[field] && inputs[field].trim() !== ''
  );

  const handleApprove = async () => {
    if (!isValid) {
      setError('Please fill in all required fields');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await onApprove(inputs);
      dismissModal();
    } catch (e) {
      const err = e as { message?: string };
      setError(err.message || 'Failed to approve step');
    } finally {
      setLoading(false);
    }
  };

  const handlePushBack = async () => {
    if (!onPushBack) return;
    if (!isValid) {
      setError('Please fill in all required fields');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await onPushBack(inputs);
      dismissModal();
    } catch (e) {
      const err = e as { message?: string };
      setError(err.message || 'Failed to push back step');
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey && isValid && !loading) {
      e.preventDefault();
      handleApprove();
    }
  };

  return (
    <Dialog open={visible} onOpenChange={dismissModal}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="text-base flex items-center gap-2">
            <span>Approve: {step.name}</span>
            {iteration > 0 && (
              <span className="text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                Iteration {iteration}
              </span>
            )}
          </DialogTitle>
        </DialogHeader>

        <div className="py-2 space-y-4">
          {/* Prompt message */}
          {prompt && (
            <div className="text-sm text-muted-foreground whitespace-pre-wrap bg-muted p-3 rounded">
              {prompt}
            </div>
          )}

          {/* Input fields */}
          {inputFields.length > 0 && (
            <div className="space-y-3">
              {inputFields.map((field) => {
                const isRequired = requiredFields.includes(field);
                const fieldId = `approval-input-${field}`;
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
                      onKeyDown={handleKeyDown}
                      className="w-full px-3 py-1 text-sm border border-border rounded bg-background focus:outline-none focus:border-ring"
                      placeholder={`Enter ${field}`}
                    />
                  </div>
                );
              })}
            </div>
          )}

          {/* No input fields message */}
          {inputFields.length === 0 && !prompt && (
            <div className="text-sm text-muted-foreground">
              Click Approve to continue the workflow execution.
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
              variant="primary"
              onClick={handlePushBack}
              disabled={loading || (!isValid && requiredFields.length > 0)}
            >
              <RotateCcw className="h-4 w-4" />
              {loading ? 'Pushing Back...' : 'Push Back'}
            </Button>
          )}
          <Button
            size="sm"
            variant="primary"
            onClick={handleApprove}
            disabled={loading || (!isValid && requiredFields.length > 0)}
          >
            <Check className="h-4 w-4" />
            {loading ? 'Approving...' : 'Approve'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
