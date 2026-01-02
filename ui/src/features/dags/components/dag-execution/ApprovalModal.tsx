import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Check, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components } from '../../../../api/v2/schema';

type Step = components['schemas']['Step'];

type WaitConfig = {
  prompt?: string;
  input?: string[];
  required?: string[];
};

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step: Step;
  onApprove: (inputs: Record<string, string>) => Promise<void>;
};

/**
 * ApprovalModal displays a form for approving a wait step with optional input fields.
 */
export function ApprovalModal({ visible, dismissModal, step, onApprove }: Props) {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Extract wait config from step's executorConfig
  const waitConfig: WaitConfig = (step.executorConfig?.config as WaitConfig) || {};
  const inputFields = waitConfig.input || [];
  const requiredFields = waitConfig.required || [];
  const prompt = waitConfig.prompt || step.description;

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

  return (
    <Dialog open={visible} onOpenChange={dismissModal}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="text-base">
            Approve: {step.name}
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
                return (
                  <div key={field}>
                    <label className="block text-sm font-medium mb-1">
                      {field}
                      {isRequired && (
                        <span className="text-error ml-1">*</span>
                      )}
                    </label>
                    <input
                      type="text"
                      value={inputs[field] || ''}
                      onChange={(e) =>
                        setInputs((prev) => ({
                          ...prev,
                          [field]: e.target.value,
                        }))
                      }
                      className="w-full px-3 py-2 text-sm border border-border rounded bg-background focus:outline-none focus:ring-2 focus:ring-primary/50"
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
          <Button
            size="sm"
            onClick={handleApprove}
            disabled={loading || (!isValid && requiredFields.length > 0)}
            className="bg-success hover:bg-success/90"
          >
            <Check className="h-4 w-4" />
            {loading ? 'Approving...' : 'Approve'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
