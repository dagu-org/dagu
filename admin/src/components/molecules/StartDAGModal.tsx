import {
  Box,
  Button,
  Modal,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import React from 'react';
import { DAG } from '../../models';
import LabeledItem from '../atoms/LabeledItem';

type Props = {
  visible: boolean;
  defaultParams: string;
  dag: DAG;
  dismissModal: () => void;
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

function StartDAGModal({
  visible,
  defaultParams,
  dag,
  dismissModal,
  onSubmit,
}: Props) {
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

  const [params, setParams] = React.useState<string>(dag.DefaultParams);

  React.useEffect(() => {
    ref.current?.focus();
  }, [ref.current]);

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
          {dag.DefaultParams != '' ? (
            <>
              <Stack direction={'column'}>
                <LabeledItem label="Default parameters">{null}</LabeledItem>
                <Box sx={{ backgroundColor: '#eee' }}>{dag.DefaultParams}</Box>
              </Stack>
              <TextField
                label="parameters"
                multiline
                variant="outlined"
                style={{
                  flex: 0.5,
                }}
                inputRef={ref}
                InputProps={{
                  value: params,
                  onChange: (e) => {
                    setParams(e.target.value);
                  },
                }}
              />
            </>
          ) : null}
          <Button variant="contained" onClick={() => onSubmit(params)}>
            Start
          </Button>
          <Button variant="contained" color="error" onClick={dismissModal}>
            Cancel
          </Button>
        </Stack>
      </Box>
    </Modal>
  );
}

export default StartDAGModal;
