/**
 * StartDAGModal component provides a modal dialog for starting or enqueuing a DAG with parameters.
 *
 * @module features/dags/components/dag-execution
 */
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import React from 'react';
import { components } from '../../../../api/v2/schema';
import {
  Parameter,
  parseParams,
  stringifyParams,
} from '../../../../lib/parseParams';

/**
 * Props for the StartDAGModal component
 */
type Props = {
  /** Whether the modal is visible */
  visible: boolean;
  /** DAG definition */
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  /** Function to close the modal */
  dismissModal: () => void;
  /** Function called when the user submits the form */
  onSubmit: (params: string, dagRunId?: string) => void;
  /** Action type: 'start' or 'enqueue' */
  action?: 'start' | 'enqueue';
};

/**
 * Modal dialog for starting or enqueuing a DAG with parameters
 */
function StartDAGModal({
  visible,
  dag,
  dismissModal,
  onSubmit,
  action = 'start',
}: Props) {
  const ref = React.useRef<HTMLInputElement>(null);

  // Parse default parameters from the DAG definition
  const parsedParams = React.useMemo(() => {
    if (!dag.defaultParams) {
      return [];
    }
    return parseParams(dag.defaultParams);
  }, [dag.defaultParams]);

  const [params, setParams] = React.useState<Parameter[]>([]);
  const [dagRunId, setDAGRunId] = React.useState<string>('');

  // Update params when default params change
  React.useEffect(() => {
    setParams(parsedParams);
  }, [parsedParams]);

  // Create refs for the buttons
  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const submitButtonRef = React.useRef<HTMLButtonElement>(null);

  // Handle keyboard events
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Only handle events when modal is visible
      if (!visible) return;

      // Handle Enter key
      if (e.key === 'Enter') {
        // Get the active element
        const activeElement = document.activeElement;

        // If Cancel button is focused, trigger cancel
        if (activeElement === cancelButtonRef.current) {
          e.preventDefault();
          dismissModal();
          return;
        }

        // If any other button is focused, let it handle the event naturally
        if (activeElement instanceof HTMLButtonElement) {
          return;
        }

        // If an input field is focused, submit the form
        const isInputFocused =
          activeElement instanceof HTMLInputElement ||
          activeElement instanceof HTMLTextAreaElement ||
          activeElement instanceof HTMLSelectElement;

        if (isInputFocused || !activeElement) {
          e.preventDefault();
          onSubmit(stringifyParams(params), dagRunId || undefined);
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, params, dagRunId, onSubmit, dismissModal]);

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>
            Start the DAG
          </DialogTitle>
        </DialogHeader>

        <div className="py-4 space-y-4">
          {/* Optional DAGRun ID field */}
          <div className="space-y-2">
            <Label htmlFor="dagRun-id">DAG-Run ID (optional)</Label>
            <Input
              id="dagRun-id"
              placeholder="Enter custom DAG-Run ID"
              value={dagRunId}
              onChange={(e) => setDAGRunId(e.target.value)}
            />
          </div>
          {parsedParams.map((p, i) => {
            if (p.Name != undefined) {
              return (
                <div key={i} className="space-y-2">
                  <Label htmlFor={`param-${i}`}>{p.Name}</Label>
                  <Input
                    id={`param-${i}`}
                    placeholder={p.Value}
                    ref={i === 0 ? ref : undefined}
                    value={params.find((pp) => pp.Name == p.Name)?.Value || ''}
                    onChange={(e) => {
                      if (p.Name) {
                        setParams(
                          params.map((pp) => {
                            if (pp.Name == p.Name) {
                              return {
                                ...pp,
                                Value: e.target.value,
                              };
                            } else {
                              return pp;
                            }
                          })
                        );
                      }
                    }}
                  />
                </div>
              );
            } else {
              return (
                <div key={i} className="space-y-2">
                  <Label htmlFor={`param-${i}`}>{`Parameter ${i + 1}`}</Label>
                  <Input
                    id={`param-${i}`}
                    placeholder={p.Value}
                    ref={i === 0 ? ref : undefined}
                    value={params.find((_, j) => i == j)?.Value || ''}
                    onChange={(e) => {
                      setParams(
                        params.map((pp, j) => {
                          if (j == i) {
                            return {
                              ...pp,
                              Value: e.target.value,
                            };
                          } else {
                            return pp;
                          }
                        })
                      );
                    }}
                  />
                </div>
              );
            }
          })}
        </div>

        <DialogFooter>
          <Button
            ref={cancelButtonRef}
            variant="outline"
            onClick={dismissModal}
          >
            Cancel
          </Button>
          <Button
            ref={submitButtonRef}
            onClick={() => {
              onSubmit(stringifyParams(params), dagRunId || undefined);
            }}
          >
            {action === 'enqueue' ? 'Enqueue' : 'Start'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default StartDAGModal;
