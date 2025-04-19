/**
 * StartDAGModal component provides a modal dialog for starting a DAG with parameters.
 *
 * @module features/dags/components/dag-execution
 */
import React from 'react';
import {
  Parameter,
  parseParams,
  stringifyParams,
} from '../../../../lib/parseParams';
import { components } from '../../../../api/v2/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

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
  onSubmit: (params: string) => void;
};

/**
 * Modal dialog for starting a DAG with parameters
 */
function StartDAGModal({ visible, dag, dismissModal, onSubmit }: Props) {
  // Handle ESC key to close the modal
  React.useEffect(() => {
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

  const ref = React.useRef<HTMLInputElement>(null);

  // Parse default parameters from the DAG definition
  const parsedParams = React.useMemo(() => {
    if (!dag.defaultParams) {
      return [];
    }
    return parseParams(dag.defaultParams);
  }, [dag.defaultParams]);

  const [params, setParams] = React.useState<Parameter[]>([]);

  // Update params when default params change
  React.useEffect(() => {
    setParams(parsedParams);
  }, [parsedParams]);

  // Don't render if modal is not visible
  if (!visible) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg border-2 border-black bg-white p-6 shadow-xl">
        <div className="flex items-center justify-center">
          <h2 className="text-xl font-semibold">Start the DAG</h2>
        </div>

        <div className="mt-4 flex flex-col space-y-4">
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

          <Button
            variant="outline"
            onClick={() => {
              onSubmit(stringifyParams(params));
            }}
          >
            Start
          </Button>

          <Button
            variant="outline"
            className="text-destructive hover:bg-destructive/10"
            onClick={dismissModal}
          >
            Cancel
          </Button>
        </div>
      </div>
    </div>
  );
}

export default StartDAGModal;
