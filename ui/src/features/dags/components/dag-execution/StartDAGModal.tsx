/**
 * StartDAGModal component provides a modal dialog for starting a DAG with parameters.
 *
 * @module features/dags/components/dag-execution
 */
import {
  Box,
  Button,
  Modal,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import React from 'react';
import {
  Parameter,
  parseParams,
  stringifyParams,
} from '../../../../lib/parseParams';
import { components } from '../../../../api/v2/schema';

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

const style = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: 400,
  bgcolor: 'background.paper',
  border: '2px solid #000',
  boxShadow: 24,
  p: 4,
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

  return (
    <Modal open={visible} onClose={dismissModal}>
      <Box sx={style}>
        <Stack direction="row" alignContent="center" justifyContent="center">
          <Typography variant="h6">Start the DAG</Typography>
        </Stack>
        <Stack
          direction="column"
          alignContent="center"
          justifyContent="center"
          spacing={2}
          mt={2}
        >
          {parsedParams.map((p, i) => {
            if (p.Name != undefined) {
              return (
                <React.Fragment key={i}>
                  <TextField
                    label={p.Name}
                    placeholder={p.Value}
                    variant="outlined"
                    style={{
                      flex: 0.5,
                    }}
                    inputRef={ref}
                    InputProps={{
                      value: params.find((pp) => pp.Name == p.Name)?.Value,
                      onChange: (e) => {
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
                      },
                    }}
                  />
                </React.Fragment>
              );
            } else {
              return (
                <React.Fragment key={i}>
                  <TextField
                    label={`Parameter ${i + 1}`}
                    placeholder={p.Value}
                    variant="outlined"
                    style={{
                      flex: 0.5,
                    }}
                    inputRef={ref}
                    InputProps={{
                      value: params.find((_, j) => i == j)?.Value,
                      onChange: (e) => {
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
                      },
                    }}
                  />
                </React.Fragment>
              );
            }
          })}
          <Button
            variant="outlined"
            onClick={() => {
              onSubmit(stringifyParams(params));
            }}
          >
            Start
          </Button>
          <Button variant="outlined" color="error" onClick={dismissModal}>
            Cancel
          </Button>
        </Stack>
      </Box>
    </Modal>
  );
}

export default StartDAGModal;
