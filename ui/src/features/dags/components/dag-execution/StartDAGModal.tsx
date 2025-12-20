/**
 * StartDAGModal component provides a modal dialog for starting or enqueuing a DAG with parameters.
 *
 * @module features/dags/components/dag-execution
 */
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
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
  onSubmit: (params: string, dagRunId?: string, immediate?: boolean) => void;
  /** Action type: 'start' or 'enqueue' */
  action?: 'start' | 'enqueue';
};

/**
 * Modal dialog for starting or enqueuing a DAG with parameters
 */
function StartDAGModal({ visible, dag, dismissModal, onSubmit }: Props) {
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
  const [enqueue, setEnqueue] = React.useState<boolean>(false);

  // Get runConfig with default values if not specified
  const dagWithRunConfig = dag as typeof dag & {
    runConfig?: { disableParamEdit?: boolean; disableRunIdEdit?: boolean };
  };

  // Determine if editing is disabled
  const paramsReadOnly = dagWithRunConfig.runConfig?.disableParamEdit ?? false;
  const runIdReadOnly = dagWithRunConfig.runConfig?.disableRunIdEdit ?? false;

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
          onSubmit(stringifyParams(params), dagRunId || undefined, !enqueue);
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, params, dagRunId, enqueue, onSubmit, dismissModal]);

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Start the DAG</DialogTitle>
        </DialogHeader>

        {(paramsReadOnly || runIdReadOnly) && (
          <div className="bg-yellow-50 border border-yellow-200 rounded-md p-3">
            <p className="text-sm text-yellow-800">
              <strong>Note:</strong> This DAG has restrictions:
              {paramsReadOnly && runIdReadOnly && (
                <span> Parameter editing and custom run IDs are disabled.</span>
              )}
              {paramsReadOnly && !runIdReadOnly && (
                <span> Parameter editing is disabled.</span>
              )}
              {!paramsReadOnly && runIdReadOnly && (
                <span> Custom run IDs are disabled.</span>
              )}
            </p>
          </div>
        )}

        <div className="py-4 space-y-4 max-h-[60vh] overflow-y-auto">
          {/* Enqueue checkbox */}
          <div className="flex items-center space-x-2">
            <Checkbox
              id="enqueue"
              checked={enqueue}
              onCheckedChange={(checked) => setEnqueue(checked as boolean)}
              className="border-gray-400"
            />
            <Label htmlFor="enqueue" className="cursor-pointer">
              Enqueue
            </Label>
          </div>
          {/* Optional DAGRun ID field */}
          <div className="space-y-2">
            <Label htmlFor="dagRun-id">DAG-Run ID (optional)</Label>
            <Input
              id="dagRun-id"
              placeholder="Enter custom DAG-Run ID"
              value={dagRunId}
              readOnly={runIdReadOnly}
              disabled={runIdReadOnly}
              className={runIdReadOnly ? 'bg-muted cursor-not-allowed' : ''}
              onChange={(e) => {
                if (!runIdReadOnly) {
                  setDAGRunId(e.target.value);
                }
              }}
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
                    readOnly={paramsReadOnly}
                    disabled={paramsReadOnly}
                    className={
                      paramsReadOnly ? 'bg-muted cursor-not-allowed' : ''
                    }
                    onChange={(e) => {
                      if (p.Name && !paramsReadOnly) {
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
                    readOnly={paramsReadOnly}
                    disabled={paramsReadOnly}
                    className={
                      paramsReadOnly ? 'bg-muted cursor-not-allowed' : ''
                    }
                    onChange={(e) => {
                      if (paramsReadOnly) return;
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
              onSubmit(
                stringifyParams(params),
                dagRunId || undefined,
                !enqueue
              );
            }}
          >
            {enqueue ? 'Enqueue' : 'Start'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default StartDAGModal;
